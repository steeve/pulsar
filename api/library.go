package api

import (
	"os"
	"fmt"
	"time"
	"errors"
	"strconv"
	"strings"
	"io/ioutil"
	"encoding/json"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/op/go-logging"
	"github.com/scakemyer/quasar/config"
	"github.com/scakemyer/quasar/xbmc"
	"github.com/scakemyer/quasar/tmdb"
)

const (
	LMovie = iota
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
	Id       string `json:"id"`
	Title    string `json:"title"`
	Year     string `json:"year"`
	Overview string `json:"overview"`
	Poster   string `json:"poster"`
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
		movie := tmdb.GetMovieById(db.Movies[i], "en")
		Movies = append(Movies, &Item{
			Id: db.Movies[i],
			Title: movie.OriginalTitle,
			Year: strings.Split(movie.ReleaseDate, "-")[0],
			Overview: movie.Overview,
			Poster: tmdb.ImageURL(movie.PosterPath, "w500"),
		})
	}

	for i := 0; i < len(db.Shows); i++ {
		showId, _ := strconv.Atoi(db.Shows[i])
		show := tmdb.GetShow(showId, "en")
		if show == nil {
			ctx.Writer.Header().Set("Access-Control-Allow-Origin", "*")
			ctx.JSON(200, gin.H{
				"success": false,
			})
			return
		}
		Shows = append(Shows, &Item{
			Id: db.Shows[i],
			Title: show.Name,
			Year: strings.Split(show.FirstAirDate, "-")[0],
			Overview: show.Overview,
			Poster: tmdb.ImageURL(show.PosterPath, "w500"),
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

	return ioutil.WriteFile(DBPath, b, 0644)
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

	return ioutil.WriteFile(DBPath, b, 0644)
}

func InJsonDB(ID string, ltype int) (bool, error) {
	var db DataBase
	LibraryPath := config.Get().LibraryPath
	DBPath := filepath.Join(LibraryPath, fmt.Sprintf("%s.json", DBName))

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
		return false, fmt.Errorf("Unknown content type")
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
			libraryLog.Error("Unable to create library path for Movies")
			ctx.String(404, "")
			return
		}
	}

	ShowsLibraryPath := filepath.Join(LibraryPath, "Shows")
	if _, err := os.Stat(ShowsLibraryPath); os.IsNotExist(err) {
		if err := os.Mkdir(ShowsLibraryPath, 0755); err != nil{
			libraryLog.Error("Unable to create library path for Shows")
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
	libraryLog.Notice("Library updated")
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

func AddMovie(ctx *gin.Context) {
	LibraryPath := config.Get().LibraryPath
	DBPath := filepath.Join(LibraryPath, fmt.Sprintf("%s.json", DBName))

	if fileInfo, err := os.Stat(LibraryPath); err != nil || fileInfo.IsDir() == false || LibraryPath == "" || LibraryPath == "." {
		xbmc.Notify("Quasar", "LOCALIZE[30220]", config.AddonIcon())
		ctx.String(404, "")
		return
	}

	tmdbId := ctx.Params.ByName("tmdbId")

	if inJsonDb, err := InJsonDB(tmdbId, LMovie); err != nil || inJsonDb == true {
		ctx.String(404, "")
		return
	}

	MoviesLibraryPath := filepath.Join(LibraryPath, "Movies")
	if _, err := os.Stat(MoviesLibraryPath); os.IsNotExist(err) {
		if err := os.Mkdir(MoviesLibraryPath, 0755); err != nil{
			libraryLog.Error("Unable to create library path for Movies")
			ctx.String(404, "")
			return
		}
	}

	if err := WriteMovieStrm(tmdbId, MoviesLibraryPath); err != nil {
		ctx.String(404, "")
		return
	}

	if err := UpdateJsonDB(DBPath, tmdbId, LMovie); err != nil {
		libraryLog.Error("Unable to update json DB")
		ctx.String(404, "")
		return
	}

	xbmc.Notify("Quasar", "LOCALIZE[30221]", config.AddonIcon())
	ctx.String(200, "")
	xbmc.VideoLibraryScan()
	ClearCache(ctx)
	xbmc.Refresh()
	libraryLog.Notice("Movie added")
}

func WriteMovieStrm(tmdbId string, MoviesLibraryPath string) error {
	movie := tmdb.GetMovieById(tmdbId, "en")
	MovieStrm := toFileName(fmt.Sprintf("%s (%s)", movie.OriginalTitle, strings.Split(movie.ReleaseDate, "-")[0]))
	MoviePath := filepath.Join(MoviesLibraryPath, MovieStrm)

	if _, err := os.Stat(MoviePath); os.IsNotExist(err) {
		if err := os.Mkdir(MoviePath, 0755); err != nil{
			libraryLog.Error("Unable to create path for Movies")
			return err
		}
	}

	MovieStrmPath := filepath.Join(MoviePath, fmt.Sprintf("%s.strm", MovieStrm))
	if err := ioutil.WriteFile(MovieStrmPath, []byte(UrlForXBMC("/library/play/movie/%s", tmdbId)), 0644); err != nil {
				libraryLog.Error("Unable to write to strm file for movie")
				return err
		}

		return nil
}

func RemoveMovie(ctx *gin.Context) {
	LibraryPath := config.Get().LibraryPath
	MoviesLibraryPath := filepath.Join(LibraryPath, "Movies")
	DBPath := filepath.Join(LibraryPath, fmt.Sprintf("%s.json", DBName))
	tmdbId := ctx.Params.ByName("tmdbId")
	movie := tmdb.GetMovieById(tmdbId, "en")
	MovieStrm := toFileName(fmt.Sprintf("%s (%s)", movie.OriginalTitle, strings.Split(movie.ReleaseDate, "-")[0]))
	MoviePath := filepath.Join(MoviesLibraryPath, MovieStrm)

	if err := RemoveFromJsonDB(DBPath, tmdbId, LMovie); err != nil {
		libraryLog.Error("Unable to remove movie from db")
		ctx.String(404, "")
		return
	}

	if err := os.RemoveAll(MoviePath); err != nil{
		libraryLog.Error("Unable to remove movie folder")
		ctx.String(404, "")
		return
	}

	xbmc.Notify("Quasar", "LOCALIZE[30222]", config.AddonIcon())
	ctx.String(200, "")
	xbmc.VideoLibraryClean()
	ClearCache(ctx)
	xbmc.Refresh()
	libraryLog.Notice("Movie removed")
}

func AddShow(ctx *gin.Context) {
	LibraryPath := config.Get().LibraryPath
	DBPath := filepath.Join(LibraryPath, fmt.Sprintf("%s.json", DBName))

	if fileInfo, err := os.Stat(LibraryPath); err != nil || fileInfo.IsDir() == false || LibraryPath == "" || LibraryPath == "." {
		xbmc.Notify("Quasar", "LOCALIZE[30220]", config.AddonIcon())
		ctx.String(404, "")
		return
	}

	showId := ctx.Params.ByName("showId")

	if inJsonDb, err := InJsonDB(showId, LShow); err != nil || inJsonDb == true {
		ctx.String(404, "")
		return
	}

	ShowsLibraryPath := filepath.Join(LibraryPath, "Shows")
	if _, err := os.Stat(ShowsLibraryPath); os.IsNotExist(err) {
		if err := os.Mkdir(ShowsLibraryPath, 0755); err != nil{
			libraryLog.Error("Unable to create library path for Shows")
			ctx.String(404, "")
			return
		}
	}

	if err := WriteShowStrm(showId, ShowsLibraryPath); err != nil {
		ctx.String(404, "")
		return
	}

	if err := UpdateJsonDB(DBPath, showId, LShow); err != nil {
			libraryLog.Error("Unable to update json DB")
			ctx.String(404, "")
			return
		}

	xbmc.Notify("Quasar", "LOCALIZE[30221]", config.AddonIcon())
	ctx.String(200, "")
	xbmc.VideoLibraryScan()
	ClearCache(ctx)
	xbmc.Refresh()
	libraryLog.Notice("Show added")
}

func WriteShowStrm(showId string, ShowsLibraryPath string) error {
	Id, _ := strconv.Atoi(showId)
	show := tmdb.GetShow(Id, "en")
	if show == nil {
		return errors.New("Unable to get Show")
	}
	ShowStrm := toFileName(fmt.Sprintf("%s (%s)", show.Name, strings.Split(show.FirstAirDate, "-")[0]))
	ShowPath := filepath.Join(ShowsLibraryPath, ShowStrm)

	if _, err := os.Stat(ShowPath); os.IsNotExist(err) {
		if err := os.Mkdir(ShowPath, 0755); err != nil {
			libraryLog.Error("Unable to create path for Show")
			return err
		}
	}

	now := time.Now().UTC()
	addSpecials := config.Get().AddSpecials

	for _, season := range show.Seasons {
		if season.EpisodeCount == 0 {
			continue
		}
		firstAired, _ := time.Parse("2006-01-02", show.FirstAirDate)
		if firstAired.After(now) {
			continue
		}
		if addSpecials == false && season.Season == 0 {
			continue
		}

		episodes := tmdb.GetSeason(Id, season.Season, "en").Episodes

		for _, episode := range episodes {
			if episode.AirDate == "" {
				continue
			}
			firstAired, _ := time.Parse("2006-01-02", episode.AirDate)
			if firstAired.After(now) {
				continue
			}

			EpisodeStrmPath := filepath.Join(ShowPath, fmt.Sprintf("%s S%02dE%02d.strm", ShowStrm, season.Season, episode.EpisodeNumber))
			playLink := UrlForXBMC("/library/play/show/%d/season/%d/episode/%d", Id, season.Season, episode.EpisodeNumber)
			if err := ioutil.WriteFile(EpisodeStrmPath, []byte(playLink), 0644); err != nil {
						libraryLog.Error("Unable to write to strm file for episode")
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
	Id, _ := strconv.Atoi(showId)
	show := tmdb.GetShow(Id, "en")
	if show == nil {
		ctx.String(404, "")
		return
	}
	ShowStrm := toFileName(fmt.Sprintf("%s (%s)", show.Name, strings.Split(show.FirstAirDate, "-")[0]))
	ShowPath := filepath.Join(ShowsLibraryPath, ShowStrm)

	if err := RemoveFromJsonDB(DBPath, showId, LShow); err != nil {
		libraryLog.Error("Unable to remove show from db")
		ctx.String(404, "")
		return
	}

	if err := os.RemoveAll(ShowPath); err != nil{
		libraryLog.Error("Unable to remove show folder")
		ctx.String(404, "")
		return
	}

	xbmc.Notify("Quasar", "LOCALIZE[30222]", config.AddonIcon())
	ctx.String(200, "")
	xbmc.VideoLibraryClean()
	ClearCache(ctx)
	xbmc.Refresh()
	libraryLog.Notice("Show removed")
}
