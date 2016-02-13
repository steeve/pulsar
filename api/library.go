package api

import (
	"os"
	"fmt"
	"time"
	"strings"
	"io/ioutil"
	"encoding/json"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/op/go-logging"
	"github.com/scakemyer/quasar/config"
	"github.com/scakemyer/quasar/xbmc"
	"github.com/scakemyer/quasar/tmdb"
	"github.com/scakemyer/quasar/tvdb"
)

const (
	imageEndpoint = "http://image.tmdb.org/t/p/"
	tvdbBanners   = "http://thetvdb.com/banners/"
	LMovie        = iota
	LShow
)

var (
	libraryLog = logging.MustGetLogger("library")
	DBName     = "QuasarDB"
)

type DataBase struct {
	Movies []string `json:"movies"`
	Shows  []string `json:"shows"`
}

type Item struct {
	Id        string `json:"id"`
	Title     string `json:"title"`
	Year      string `json:"year"`
	Overview  string `json:"overview"`
	Poster    string `json:"poster"`
}

func toFileName(filename string) string {
	reserved := []string{"<", ">", ":", "\"", "/", "\\", "|", "?", "*", "%", "+"}
	for _, reservedchar := range reserved {
		filename = strings.Replace(filename, reservedchar, "", -1)
	}
	return filename
}

func PlayMovie(ctx *gin.Context) {
	if config.Get().ChooseStreamAuto == true {
		MoviePlay(ctx)
	} else {
		MovieLinks(ctx)
	}
}

func PlayShow(ctx *gin.Context) {
	if config.Get().ChooseStreamAuto == true {
		ShowEpisodePlay(ctx)
	} else {
		ShowEpisodeLinks(ctx)
	}
}

func Lookup(ctx *gin.Context) {
	var db DataBase

	LibraryPath := config.Get().LibraryPath
	DBPath := filepath.Join(LibraryPath, fmt.Sprintf("%s.json", DBName))

	if _, err := os.Stat(DBPath); err == nil {
		file, err := ioutil.ReadFile(DBPath)
	    if err != nil {
	    	ctx.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	        ctx.JSON(200, gin.H{
	            "success": false,
	        })
	        return
	    }
	    json.Unmarshal(file, &db)
	}

	Movies := make([]*Item, 0, len(db.Movies))
	Shows := make([]*Item, 0, len(db.Shows))

	for i := 0; i < len(db.Movies); i++ {
		movie := tmdb.GetMovieFromIMDB(db.Movies[i], "en")
		Movies = append(Movies, &Item{
			Id: db.Movies[i],
			Title: movie.OriginalTitle,
			Year: strings.Split(movie.ReleaseDate, "-")[0],
			Overview: movie.Overview,
			Poster: imageEndpoint + "w500" + movie.PosterPath,
		})
	}

	for i := 0; i < len(db.Shows); i++ {
		show, err := tvdb.NewShowCached(db.Shows[i], "en")
		if err != nil {
			ctx.Writer.Header().Set("Access-Control-Allow-Origin", "*")
			ctx.JSON(200, gin.H{
	            "success": false,
	        })
	        return
		}
		Shows = append(Shows, &Item{
			Id: db.Shows[i],
			Title: show.SeriesName,
			Year: strings.Split(show.FirstAired, "-")[0],
			Overview: show.Overview,
			Poster: tvdbBanners + show.Poster,
		})
	}

	ctx.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	ctx.JSON(200, gin.H{
		"success": true,
		"results": gin.H{
			"movies": Movies,
			"shows": Shows,
		},
	})
}

func UpdateJsonDB(DBPath string, ID string, ltype int) error {
	var db DataBase

	if _, err := os.Stat(DBPath); err == nil {
		file, err := ioutil.ReadFile(DBPath)
	    if err != nil {
	        return err
	    }
	    json.Unmarshal(file, &db)
	}

    if ltype == LMovie {
    	db.Movies = append(db.Movies, ID)
    } else if ltype == LShow {
    	db.Shows = append(db.Shows, ID)
    } else {
    	return fmt.Errorf("Unknown ltype")
    }
    b, err := json.Marshal(db)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(DBPath, b, 0755)
}

func RemoveFromJsonDB(DBPath string, ID string, ltype int) error {
	var db DataBase
	var new_db DataBase

	if _, err := os.Stat(DBPath); err == nil {
		file, err := ioutil.ReadFile(DBPath)
	    if err != nil {
	        return err
	    }
	    json.Unmarshal(file, &db)
	}

	for _, movieId := range db.Movies {
		if ltype == LMovie && movieId == ID {
			continue
		}
		new_db.Movies = append(new_db.Movies, movieId)
	}
	for _, showId := range db.Shows {
		if ltype == LShow && showId == ID {
			continue
		}
		new_db.Shows = append(new_db.Shows, showId)
	}

	b, err := json.Marshal(new_db)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(DBPath, b, 0755)
}

func inJsonDB(DBPath string, ID string, ltype int) (bool, error) {
	var db DataBase

	if _, err := os.Stat(DBPath); err == nil {
		file, err := ioutil.ReadFile(DBPath)
	    if err != nil {
	        return false, err
	    }
	    json.Unmarshal(file, &db)
	}

	if ltype == LMovie {
    	for _, movieId := range db.Movies {
    		if movieId == ID {
    			return true, nil
    		}
    	}
    } else if ltype == LShow {
    	for _, showId := range db.Shows {
    		if showId == ID {
    			return true, nil
    		}
    	}
    } else {
    	return false, fmt.Errorf("Unknown ltype")
    }

    return false, nil
}

func UpdateLibrary(ctx *gin.Context) {
	LibraryPath := config.Get().LibraryPath
	DBPath := filepath.Join(LibraryPath, fmt.Sprintf("%s.json", DBName))

  if fileInfo, err := os.Stat(LibraryPath); err != nil || fileInfo.IsDir() == false || LibraryPath == "" || LibraryPath == "." {
		ctx.String(404, "")
		return
	}

	if _, err := os.Stat(DBPath); err != nil {
		ctx.String(404, "")
		return
	}

	MoviesLibraryPath := filepath.Join(LibraryPath, "Movies")
	if _, err := os.Stat(MoviesLibraryPath); os.IsNotExist(err) {
		if err := os.Mkdir(MoviesLibraryPath, 0755); err != nil{
			libraryLog.Info("Unable to create MoviesLibraryPath")
			ctx.String(404, "")
			return
		}
	}

	ShowsLibraryPath := filepath.Join(LibraryPath, "Shows")
	if _, err := os.Stat(ShowsLibraryPath); os.IsNotExist(err) {
		if err := os.Mkdir(ShowsLibraryPath, 0755); err != nil{
			libraryLog.Info("Unable to create ShowsLibraryPath")
			ctx.String(404, "")
			return
		}
	}

	var db DataBase
	file, err := ioutil.ReadFile(DBPath)
    if err != nil {
    	ctx.String(404, "")
        return
    }
    json.Unmarshal(file, &db)

    for _, movieId := range db.Movies {
			WriteMovieStrm(movieId, MoviesLibraryPath)
    }

    for _, showId := range db.Shows {
    	WriteShowStrm(showId, ShowsLibraryPath)
    }

    ctx.String(200, "")
    xbmc.VideoLibraryScan()
    libraryLog.Info("Library Updated")
}

func GetLibraryPath(ctx *gin.Context) {
	LibraryPath := config.Get().LibraryPath
	ctx.String(200, LibraryPath)
}

func GetCount(ctx *gin.Context) {
	LibraryPath := config.Get().LibraryPath
	DBPath := filepath.Join(LibraryPath, fmt.Sprintf("%s.json", DBName))
	var db DataBase

	if _, err := os.Stat(DBPath); err == nil {
		file, err := ioutil.ReadFile(DBPath)
	    if err != nil {
	    	ctx.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	    	ctx.JSON(200, gin.H{
	            "success": false,
	        })
	        return
	    }
	    json.Unmarshal(file, &db)
	}

	ctx.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	ctx.JSON(200, gin.H{
        "success": true,
        "movies": len(db.Movies),
        "shows": len(db.Shows),
        "total": len(db.Movies) + len(db.Shows),
    })
}

func AddRemoveMovie(ctx *gin.Context) {
	LibraryPath := config.Get().LibraryPath
	DBPath := filepath.Join(LibraryPath, fmt.Sprintf("%s.json", DBName))
	imdbId := ctx.Params.ByName("imdbId")
	inJsonDb, err := inJsonDB(DBPath, imdbId, LMovie)
	if err != nil {
		return
	}
	if inJsonDb == true {
		RemoveMovie(ctx)
	} else {
		AddMovie(ctx)
	}
}

func AddMovie(ctx *gin.Context) {
	LibraryPath := config.Get().LibraryPath
	DBPath := filepath.Join(LibraryPath, fmt.Sprintf("%s.json", DBName))
	imdbId := ctx.Params.ByName("imdbId")

	if fileInfo, err := os.Stat(LibraryPath); err != nil || fileInfo.IsDir() == false || LibraryPath == "" || LibraryPath == "." {
		xbmc.Notify("Quasar", "LOCALIZE[30220]", config.AddonIcon())
		ctx.String(404, "")
		return
	}

	if inJsonDb, err := inJsonDB(DBPath, imdbId, LMovie); err != nil || inJsonDb == true {
		ctx.String(404, "")
		return
	}

	MoviesLibraryPath := filepath.Join(LibraryPath, "Movies")
	if _, err := os.Stat(MoviesLibraryPath); os.IsNotExist(err) {
		if err := os.Mkdir(MoviesLibraryPath, 0755); err != nil{
			libraryLog.Info("Unable to create MoviesLibraryPath")
			ctx.String(404, "")
			return
		}
	}

	if err := WriteMovieStrm(imdbId, MoviesLibraryPath); err != nil {
		ctx.String(404, "")
		return
	}

    if err := UpdateJsonDB(DBPath, imdbId, LMovie); err != nil {
			libraryLog.Info("Unable to UpdateJsonDB")
    	ctx.String(404, "")
    	return
    }

	xbmc.Notify("Quasar", "LOCALIZE[30221]", config.AddonIcon())
	ctx.String(200, "")
	xbmc.VideoLibraryScan()
	libraryLog.Info("Movie added")
}

func WriteMovieStrm(imdbId string, MoviesLibraryPath string) error {
	movie := tmdb.GetMovieFromIMDB(imdbId, "en")
	MovieStrm := toFileName(fmt.Sprintf("%s (%s)", movie.OriginalTitle, strings.Split(movie.ReleaseDate, "-")[0]))
	MoviePath := filepath.Join(MoviesLibraryPath, MovieStrm)

	if _, err := os.Stat(MoviePath); os.IsNotExist(err) {
		if err := os.Mkdir(MoviePath, 0755); err != nil{
			libraryLog.Info("Unable to create MoviePath")
			return err
		}
	}

	MovieStrmPath := filepath.Join(MoviePath, fmt.Sprintf("%s.strm", MovieStrm))
	if err := ioutil.WriteFile(MovieStrmPath, []byte(UrlForXBMC("/library/play/movie/%s", imdbId)), 0755); err != nil {
        libraryLog.Info("Unable to write to MovieStrmPath")
        return err
    }

    return nil
}

func RemoveMovie(ctx *gin.Context) {
	LibraryPath := config.Get().LibraryPath
	MoviesLibraryPath := filepath.Join(LibraryPath, "Movies")
	DBPath := filepath.Join(LibraryPath, fmt.Sprintf("%s.json", DBName))
	imdbId := ctx.Params.ByName("imdbId")
	movie := tmdb.GetMovieFromIMDB(imdbId, "en")
	MovieStrm := toFileName(fmt.Sprintf("%s (%s)", movie.OriginalTitle, strings.Split(movie.ReleaseDate, "-")[0]))
	MoviePath := filepath.Join(MoviesLibraryPath, MovieStrm)

	if err := RemoveFromJsonDB(DBPath, imdbId, LMovie); err != nil {
		libraryLog.Info("Unable to remove movie from db")
		ctx.String(404, "")
		return
	}

	if err := os.RemoveAll(MoviePath); err != nil{
		libraryLog.Info("Unable to remove movie folder")
		ctx.String(404, "")
		return
	}

	xbmc.Notify("Quasar", "LOCALIZE[30222]", config.AddonIcon())
	ctx.String(200, "")
	xbmc.VideoLibraryClean()
	libraryLog.Info("Movie removed")
}

func AddRemoveShow(ctx *gin.Context) {
	LibraryPath := config.Get().LibraryPath
	DBPath := filepath.Join(LibraryPath, fmt.Sprintf("%s.json", DBName))
	showId := ctx.Params.ByName("showId")
	inJsonDb, err := inJsonDB(DBPath, showId, LShow)
	if err != nil {
		return
	}
	if inJsonDb == true {
		RemoveShow(ctx)
	} else {
		AddShow(ctx)
	}
}

func AddShow(ctx *gin.Context) {
	LibraryPath := config.Get().LibraryPath
	DBPath := filepath.Join(LibraryPath, fmt.Sprintf("%s.json", DBName))
	showId := ctx.Params.ByName("showId")

	if fileInfo, err := os.Stat(LibraryPath); err != nil || fileInfo.IsDir() == false || LibraryPath == "" || LibraryPath == "." {
		xbmc.Notify("Quasar", "LOCALIZE[30220]", config.AddonIcon())
		ctx.String(404, "")
		return
	}

	if inJsonDb, err := inJsonDB(DBPath, showId, LShow); err != nil || inJsonDb == true {
		ctx.String(404, "")
		return
	}

	ShowsLibraryPath := filepath.Join(LibraryPath, "Shows")
	if _, err := os.Stat(ShowsLibraryPath); os.IsNotExist(err) {
		if err := os.Mkdir(ShowsLibraryPath, 0755); err != nil{
			libraryLog.Info("Unable to create ShowsLibraryPath")
			ctx.String(404, "")
			return
		}
	}

	if err := WriteShowStrm(showId, ShowsLibraryPath); err != nil {
		ctx.String(404, "")
		return
	}

	if err := UpdateJsonDB(DBPath, showId, LShow); err != nil {
			libraryLog.Info("Unable to UpdateJsonDB")
    	ctx.String(404, "")
    	return
    }

	xbmc.Notify("Quasar", "LOCALIZE[30221]", config.AddonIcon())
	ctx.String(200, "")
	xbmc.VideoLibraryScan()
	libraryLog.Info("Show added")
}

func WriteShowStrm(showId string, ShowsLibraryPath string) error {
	show, err := tvdb.NewShow(showId, "en")
	if err != nil {
		return err
	}
	ShowPath := filepath.Join(ShowsLibraryPath, toFileName(show.SeriesName))

	if _, err := os.Stat(ShowPath); os.IsNotExist(err) {
		if err := os.Mkdir(ShowPath, 0755); err != nil {
			libraryLog.Info("Unable to create ShowPath")
			return err
		}
	}

	now := time.Now().UTC()
	for _, season := range show.Seasons {
		if len(season.Episodes) == 0 {
			continue
		}
		airedDateTime := fmt.Sprintf("%s %s EST", season.Episodes[0].FirstAired, show.AirsTime)
		firstAired, _ := time.Parse("2006-01-02 3:04 PM MST", airedDateTime)
		if firstAired.Add(time.Duration(show.Runtime) * time.Minute).After(now) {
			continue
		}

		/*var SeasonPath string
		if season.Season == 0 {
			SeasonPath = filepath.Join(ShowPath, "Specials")
		} else {
			SeasonPath = filepath.Join(ShowPath, fmt.Sprintf("Season %d", season.Season))
		}

		if _, err := os.Stat(SeasonPath); os.IsNotExist(err) {
			if err := os.Mkdir(SeasonPath, 0755); err != nil {
				libraryLog.Info("Unable to create SeasonPath")
				return err
			}
		}*/

		for _, episode := range season.Episodes {
			if episode.FirstAired == "" {
				continue
			}
			airedDateTime := fmt.Sprintf("%s %s EST", episode.FirstAired, show.AirsTime)
			firstAired, _ := time.Parse("2006-01-02 3:04 PM MST", airedDateTime)
			if firstAired.Add(time.Duration(show.Runtime) * time.Minute).After(now) {
				continue
			}

			EpisodeStrmPath := filepath.Join(ShowPath, fmt.Sprintf("%s S%02dE%02d.strm", toFileName(show.SeriesName), season.Season, episode.EpisodeNumber))
			playLink := UrlForXBMC("/library/play/show/%s/season/%d/episode/%d", showId, season.Season, episode.EpisodeNumber)
			if err := ioutil.WriteFile(EpisodeStrmPath, []byte(playLink), 0755); err != nil {
		        libraryLog.Info("Unable to write to EpisodeStrmPath")
		        return err
		    }
		}
	}

	return nil
}

func RemoveShow(ctx *gin.Context) {
	LibraryPath := config.Get().LibraryPath
	ShowsLibraryPath := filepath.Join(LibraryPath, "Shows")
	DBPath := filepath.Join(LibraryPath, fmt.Sprintf("%s.json", DBName))
	showId := ctx.Params.ByName("showId")
	show, err := tvdb.NewShow(showId, "en")
	if err != nil {
		ctx.String(404, "")
		return
	}
	ShowPath := filepath.Join(ShowsLibraryPath, toFileName(show.SeriesName))

	if err := RemoveFromJsonDB(DBPath, showId, LShow); err != nil {
		libraryLog.Info("Unable to remove show from db")
		ctx.String(404, "")
		return
	}

	if err := os.RemoveAll(ShowPath); err != nil{
		libraryLog.Info("Unable to remove show folder")
		ctx.String(404, "")
		return
	}

	xbmc.Notify("Quasar", "LOCALIZE[30222]", config.AddonIcon())
	ctx.String(200, "")
	xbmc.VideoLibraryClean()
	libraryLog.Info("Show removed")
}
