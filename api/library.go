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
	"github.com/scakemyer/quasar/util"
	"github.com/scakemyer/quasar/xbmc"
	"github.com/scakemyer/quasar/tmdb"
	"github.com/scakemyer/quasar/trakt"
	"github.com/scakemyer/quasar/config"
	"github.com/scakemyer/quasar/bittorrent"
)

const (
	LMovie = iota
	LShow
)

const (
	TVDBScraper = iota
	TMDBScraper
	TraktScraper
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

func checkLibraryPath(LibraryPath string) bool {
	if fileInfo, err := os.Stat(LibraryPath); err != nil || fileInfo.IsDir() == false || LibraryPath == "" || LibraryPath == "." {
		xbmc.Notify("Quasar", "LOCALIZE[30220]", config.AddonIcon())
		return false
	}
	return true
}

func checkMoviesPath(MoviesLibraryPath string) bool {
	if _, err := os.Stat(MoviesLibraryPath); os.IsNotExist(err) {
		if err := os.Mkdir(MoviesLibraryPath, 0755); err != nil {
			libraryLog.Error("Unable to create library path for Movies")
			return false
		}
	}
	return true
}

func checkShowsPath(ShowsLibraryPath string) bool {
	if _, err := os.Stat(ShowsLibraryPath); os.IsNotExist(err) {
		if err := os.Mkdir(ShowsLibraryPath, 0755); err != nil {
			libraryLog.Error("Unable to create library path for Shows")
			return false
		}
	}
	return true
}

func isDuplicateMovie(imdbID string, libraryMovies *xbmc.VideoLibraryMovies) bool {
	for _, movie := range libraryMovies.Movies {
		if movie.IMDBNumber != "" {
			if movie.IMDBNumber == imdbID {
				return true
			}
		}
	}
	return false
}

func isDuplicateShow(imdbID string, libraryShows *xbmc.VideoLibraryShows) bool {
	for _, show := range libraryShows.Shows {
		if show.IMDBNumber != "" {
			if show.IMDBNumber == imdbID {
				return true
			}
		}
	}
	return false
}

func PlayMovie(btService *bittorrent.BTService) gin.HandlerFunc {
	if config.Get().ChooseStreamAuto == true {
		return MoviePlay(btService)
	} else {
		return MovieLinks(btService)
	}
}

func PlayShow(btService *bittorrent.BTService) gin.HandlerFunc {
	if config.Get().ChooseStreamAuto == true {
		return ShowEpisodePlay(btService)
	} else {
		return ShowEpisodeLinks(btService)
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

	if checkLibraryPath(LibraryPath) == false {
		ctx.String(200, "Library path error")
		return
	}

	if _, err := os.Stat(DBPath); err != nil {
		ctx.String(200, "DB path error")
		return
	}

	MoviesLibraryPath := filepath.Join(LibraryPath, "Movies")
	if checkMoviesPath(MoviesLibraryPath) == false {
		ctx.String(200, "Library path error for Movies")
		return
	}

	ShowsLibraryPath := filepath.Join(LibraryPath, "Shows")
	if checkShowsPath(ShowsLibraryPath) == false {
		ctx.String(200, "Library path error for Shows")
		return
	}

	var db DataBase
	file, err := ioutil.ReadFile(DBPath)
	if err != nil {
		ctx.String(200, err.Error())
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

func GetUserLists(ctx *gin.Context) {
	ctx.JSON(200, trakt.Userlists())
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
	tmdbId := ctx.Params.ByName("tmdbId")
	LibraryPath := config.Get().LibraryPath
	DBPath := filepath.Join(LibraryPath, fmt.Sprintf("%s.json", DBName))

	if checkLibraryPath(LibraryPath) == false {
		ctx.String(200, "Library path error")
		return
	}

	if inJsonDb, err := InJsonDB(tmdbId, LMovie); err != nil || inJsonDb == true {
		ctx.String(200, "Movie already in library")
		return
	}

	// Duplicate check against Kodi library
	if config.Get().IgnoreDuplicates == true {
		libraryMovies := xbmc.VideoLibraryGetMovies()
		movie := tmdb.GetMovieById(tmdbId, "en")
		if isDuplicateMovie(movie.IMDBId, libraryMovies) {
			libraryLog.Warningf("%s (%s) already in library", movie.Title, movie.IMDBId)
			xbmc.Notify("Quasar", "LOCALIZE[30265]", config.AddonIcon())
			return
		}
	}

	MoviesLibraryPath := filepath.Join(LibraryPath, "Movies")
	if checkMoviesPath(MoviesLibraryPath) == false {
		ctx.String(200, "Library path error")
		return
	}

	if err := WriteMovieStrm(tmdbId, MoviesLibraryPath); err != nil {
		ctx.String(200, "Library path error for Movies")
		return
	}

	if err := UpdateJsonDB(DBPath, tmdbId, LMovie); err != nil {
		ctx.String(200, "Unable to update json DB")
		return
	}

	xbmc.Notify("Quasar", "LOCALIZE[30221]", config.AddonIcon())
	libraryLog.Notice("Movie added")
	time.Sleep(5 * time.Second)
	xbmc.VideoLibraryScan()
	ctx.String(200, "")
}

func AddMovieList(ctx *gin.Context) {
	listId := ctx.Params.ByName("listId")
	updating := ctx.DefaultQuery("updating", "false")

	LibraryPath := config.Get().LibraryPath
	DBPath := filepath.Join(LibraryPath, fmt.Sprintf("%s.json", DBName))

	if checkLibraryPath(LibraryPath) == false {
		ctx.String(200, "Library path error")
		return
	}

	MoviesLibraryPath := filepath.Join(LibraryPath, "Movies")
	if checkMoviesPath(MoviesLibraryPath) == false {
		ctx.String(200, "Library path error for Movies")
		return
	}

	movies, err := trakt.ListItemsMovies(listId, "0")
	if err != nil {
		libraryLog.Error(err)
		ctx.String(200, err.Error())
		return
	}

	libraryMovies := xbmc.VideoLibraryGetMovies()

	for _, movie := range movies {
		title := movie.Movie.Title
		if movie.Movie.IDs.TMDB == 0 {
			libraryLog.Warningf("Missing TMDB ID for %s", title)
			continue
		}
		tmdbId := strconv.Itoa(movie.Movie.IDs.TMDB)
		if updating == "false" && config.Get().IgnoreDuplicates == true {
			if inJsonDb, err := InJsonDB(tmdbId, LMovie); err != nil || inJsonDb == true {
				libraryLog.Warningf("%s already in library", title)
				continue
			}
			if isDuplicateMovie(movie.Movie.IDs.IMDB, libraryMovies) {
				libraryLog.Warningf("%s (%s) already in library", title, movie.Movie.IDs.IMDB)
				continue
			}
		}
		if err := WriteMovieStrm(tmdbId, MoviesLibraryPath); err != nil {
			libraryLog.Errorf("Unable to write strm file for %s", title)
			continue
		}
		if err := UpdateJsonDB(DBPath, tmdbId, LMovie); err != nil {
			libraryLog.Error("Unable to update json DB")
			continue
		}
	}

	if updating == "false" {
		xbmc.Notify("Quasar", "LOCALIZE[30221]", config.AddonIcon())
	}
	libraryLog.Notice("Movie list added")
	ctx.String(200, "")
}

func AddMovieCollection(ctx *gin.Context) {
	updating := ctx.DefaultQuery("updating", "false")

	LibraryPath := config.Get().LibraryPath
	DBPath := filepath.Join(LibraryPath, fmt.Sprintf("%s.json", DBName))

	if checkLibraryPath(LibraryPath) == false {
		ctx.String(200, "Library path error")
		return
	}

	MoviesLibraryPath := filepath.Join(LibraryPath, "Movies")
	if checkMoviesPath(MoviesLibraryPath) == false {
		ctx.String(200, "Library path error for Movies")
		return
	}

	movies, err := trakt.CollectionMovies()
	if err != nil {
		libraryLog.Error(err)
		ctx.String(200, err.Error())
		return
	}

	libraryMovies := xbmc.VideoLibraryGetMovies()

	for _, movie := range movies {
		title := movie.Movie.Title
		if movie.Movie.IDs.TMDB == 0 {
			libraryLog.Warningf("Missing TMDB ID for %s", title)
			continue
		}
		tmdbId := strconv.Itoa(movie.Movie.IDs.TMDB)
		if updating == "false" && config.Get().IgnoreDuplicates == true {
			if inJsonDb, err := InJsonDB(tmdbId, LMovie); err != nil || inJsonDb == true {
				libraryLog.Warningf("%s already in library", title)
				continue
			}
			if isDuplicateMovie(movie.Movie.IDs.IMDB, libraryMovies) {
				libraryLog.Warningf("%s (%s) already in library", title, movie.Movie.IDs.IMDB)
				continue
			}
		}
		if err := WriteMovieStrm(tmdbId, MoviesLibraryPath); err != nil {
			libraryLog.Errorf("Unable to write strm file for %s", title)
			continue
		}
		if err := UpdateJsonDB(DBPath, tmdbId, LMovie); err != nil {
			libraryLog.Error("Unable to update json DB")
			continue
		}
	}

	if updating == "false" {
		xbmc.Notify("Quasar", "LOCALIZE[30221]", config.AddonIcon())
	}
	libraryLog.Notice("Movie collection added")
	ctx.String(200, "")
}

func AddMovieWatchlist(ctx *gin.Context) {
	updating := ctx.DefaultQuery("updating", "false")

	LibraryPath := config.Get().LibraryPath
	DBPath := filepath.Join(LibraryPath, fmt.Sprintf("%s.json", DBName))

	if checkLibraryPath(LibraryPath) == false {
		ctx.String(200, "Library path error")
		return
	}

	MoviesLibraryPath := filepath.Join(LibraryPath, "Movies")
	if checkMoviesPath(MoviesLibraryPath) == false {
		ctx.String(200, "Library path error for Movies")
		return
	}

	movies, err := trakt.WatchlistMovies()
	if err != nil {
		libraryLog.Error(err)
		ctx.String(200, err.Error())
		return
	}

	libraryMovies := xbmc.VideoLibraryGetMovies()

	for _, movie := range movies {
		title := movie.Movie.Title
		if movie.Movie.IDs.TMDB == 0 {
			libraryLog.Warningf("Missing TMDB ID for %s", title)
			continue
		}
		tmdbId := strconv.Itoa(movie.Movie.IDs.TMDB)
		if updating == "false" && config.Get().IgnoreDuplicates == true {
			if inJsonDb, err := InJsonDB(tmdbId, LMovie); err != nil || inJsonDb == true {
				libraryLog.Warningf("%s already in library", title)
				continue
			}
			if isDuplicateMovie(movie.Movie.IDs.IMDB, libraryMovies) {
				libraryLog.Warningf("%s (%s) already in library", title, movie.Movie.IDs.IMDB)
				continue
			}
		}
		if err := WriteMovieStrm(tmdbId, MoviesLibraryPath); err != nil {
			libraryLog.Errorf("Unable to write strm file for %s", title)
			continue
		}
		if err := UpdateJsonDB(DBPath, tmdbId, LMovie); err != nil {
			libraryLog.Error("Unable to update json DB")
			continue
		}
	}

	if updating == "false" {
		xbmc.Notify("Quasar", "LOCALIZE[30221]", config.AddonIcon())
	}
	libraryLog.Notice("Movie watchlist added")
	ctx.String(200, "")
}

func WriteMovieStrm(tmdbId string, MoviesLibraryPath string) error {
	movie := tmdb.GetMovieById(tmdbId, "en")
	if movie == nil {
		return errors.New("Unable to get movie")
	}
	MovieStrm := toFileName(fmt.Sprintf("%s (%s)", movie.OriginalTitle, strings.Split(movie.ReleaseDate, "-")[0]))
	MoviePath := filepath.Join(MoviesLibraryPath, MovieStrm)

	if _, err := os.Stat(MoviePath); os.IsNotExist(err) {
		if err := os.Mkdir(MoviePath, 0755); err != nil {
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
		ctx.String(200, "Unable to remove movie from db")
		return
	}

	if err := os.RemoveAll(MoviePath); err != nil {
		ctx.String(200, "Unable to remove movie folder")
		return
	}

	xbmc.Notify("Quasar", "LOCALIZE[30222]", config.AddonIcon())
	libraryLog.Notice("Movie removed")
	ctx.String(200, "")
	xbmc.Refresh()
}

func AddShow(ctx *gin.Context) {
	tmdbId := ctx.Params.ByName("tmdbId")
	LibraryPath := config.Get().LibraryPath
	DBPath := filepath.Join(LibraryPath, fmt.Sprintf("%s.json", DBName))

	if checkLibraryPath(LibraryPath) == false {
		ctx.String(200, "Library path error")
		return
	}

	if inJsonDb, err := InJsonDB(tmdbId, LShow); err != nil || inJsonDb == true {
		ctx.String(200, "Show already in library")
		return
	}

	// Duplicate check against Kodi library
	if config.Get().IgnoreDuplicates == true {
		libraryShows := xbmc.VideoLibraryGetShows()
		Id, _ := strconv.Atoi(tmdbId)
		show := tmdb.GetShow(Id, "en")
		showId := tmdbId
		if config.Get().TvScraper == TVDBScraper {
			if show.ExternalIDs != nil {
				showId = strconv.Itoa(util.StrInterfaceToInt(show.ExternalIDs.TVDBID))
			}
		}
		if isDuplicateShow(showId, libraryShows) {
			libraryLog.Warningf("%s (%s) already in library", show.Name, showId)
			xbmc.Notify("Quasar", "LOCALIZE[30265]", config.AddonIcon())
			return
		}
	}

	ShowsLibraryPath := filepath.Join(LibraryPath, "Shows")
	if checkShowsPath(ShowsLibraryPath) == false {
		ctx.String(200, "Library path error")
		return
	}

	if err := WriteShowStrm(tmdbId, ShowsLibraryPath); err != nil {
		ctx.String(200, "Error writing strm")
		return
	}

	if err := UpdateJsonDB(DBPath, tmdbId, LShow); err != nil {
		ctx.String(200, "Unable to update json DB")
		return
	}

	xbmc.Notify("Quasar", "LOCALIZE[30221]", config.AddonIcon())
	libraryLog.Notice("Show added")
	time.Sleep(5 * time.Second)
	xbmc.VideoLibraryScan()
	ctx.String(200, "")
}

func AddShowList(ctx *gin.Context) {
	listId := ctx.Params.ByName("listId")
	updating := ctx.DefaultQuery("updating", "false")

	LibraryPath := config.Get().LibraryPath
	DBPath := filepath.Join(LibraryPath, fmt.Sprintf("%s.json", DBName))

	if checkLibraryPath(LibraryPath) == false {
		ctx.String(200, "Library path error")
		return
	}

	ShowsLibraryPath := filepath.Join(LibraryPath, "Shows")
	if checkShowsPath(ShowsLibraryPath) == false {
		ctx.String(200, "Library path error for Shows")
		return
	}

	shows, err := trakt.ListItemsShows(listId, "0")
	if err != nil {
		libraryLog.Error(err)
		ctx.String(200, err.Error())
		return
	}

	libraryShows := xbmc.VideoLibraryGetShows()

	for _, show := range shows {
		title := show.Show.Title
		if show.Show.IDs.TMDB == 0 {
			libraryLog.Warningf("Missing TMDB ID for %s", title)
			continue
		}
		tmdbId := strconv.Itoa(show.Show.IDs.TMDB)
		if updating == "false" && config.Get().IgnoreDuplicates == true {
			if inJsonDb, err := InJsonDB(tmdbId, LShow); err != nil || inJsonDb == true {
				libraryLog.Warningf("%s already in library", title)
				continue
			}
			showId := tmdbId
			if config.Get().TvScraper == TVDBScraper {
					showId = strconv.Itoa(show.Show.IDs.TVDB)
			}
			if isDuplicateShow(showId, libraryShows) {
				libraryLog.Warningf("%s (%s) already in library", title, showId)
				continue
			}
		}
		if err := WriteShowStrm(tmdbId, ShowsLibraryPath); err != nil {
			libraryLog.Errorf("Unable to write strm file for %s", title)
			continue
		}
		if err := UpdateJsonDB(DBPath, tmdbId, LShow); err != nil {
			libraryLog.Error("Unable to update json DB")
			continue
		}
	}

	if updating == "false" {
		xbmc.Notify("Quasar", "LOCALIZE[30221]", config.AddonIcon())
	}
	libraryLog.Notice("Show list added")
	ctx.String(200, "")
}

func AddShowCollection(ctx *gin.Context) {
	updating := ctx.DefaultQuery("updating", "false")

	LibraryPath := config.Get().LibraryPath
	DBPath := filepath.Join(LibraryPath, fmt.Sprintf("%s.json", DBName))

	if checkLibraryPath(LibraryPath) == false {
		ctx.String(200, "Library path error")
		return
	}

	ShowsLibraryPath := filepath.Join(LibraryPath, "Shows")
	if checkShowsPath(ShowsLibraryPath) == false {
		ctx.String(200, "Library path error for Shows")
		return
	}

	shows, err := trakt.CollectionShows()
	if err != nil {
		libraryLog.Error(err)
		ctx.String(200, err.Error())
		return
	}

	libraryShows := xbmc.VideoLibraryGetShows()

	for _, show := range shows {
		title := show.Show.Title
		if show.Show.IDs.TMDB == 0 {
			libraryLog.Warningf("Missing TMDB ID for %s", title)
			continue
		}
		tmdbId := strconv.Itoa(show.Show.IDs.TMDB)
		if updating == "false" && config.Get().IgnoreDuplicates == true {
			if inJsonDb, err := InJsonDB(tmdbId, LShow); err != nil || inJsonDb == true {
				libraryLog.Warningf("%s already in library", title)
				continue
			}
			showId := tmdbId
			if config.Get().TvScraper == TVDBScraper {
					showId = strconv.Itoa(show.Show.IDs.TVDB)
			}
			if isDuplicateShow(showId, libraryShows) {
				libraryLog.Warningf("%s (%s) already in library", title, showId)
				continue
			}
		}
		if err := WriteShowStrm(tmdbId, ShowsLibraryPath); err != nil {
			libraryLog.Errorf("Unable to write strm file for %s", title)
			continue
		}
		if err := UpdateJsonDB(DBPath, tmdbId, LShow); err != nil {
			libraryLog.Error("Unable to update json DB")
			continue
		}
	}

	if updating == "false" {
		xbmc.Notify("Quasar", "LOCALIZE[30221]", config.AddonIcon())
	}
	libraryLog.Notice("Show collection added")
	ctx.String(200, "")
}

func AddShowWatchlist(ctx *gin.Context) {
	updating := ctx.DefaultQuery("updating", "false")

	LibraryPath := config.Get().LibraryPath
	DBPath := filepath.Join(LibraryPath, fmt.Sprintf("%s.json", DBName))

	if checkLibraryPath(LibraryPath) == false {
		ctx.String(200, "Library path error")
		return
	}

	ShowsLibraryPath := filepath.Join(LibraryPath, "Shows")
	if checkShowsPath(ShowsLibraryPath) == false {
		ctx.String(200, "Library path error for Shows")
		return
	}

	shows, err := trakt.WatchlistShows()
	if err != nil {
		libraryLog.Error(err)
		ctx.String(200, err.Error())
		return
	}

	libraryShows := xbmc.VideoLibraryGetShows()

	for _, show := range shows {
		title := show.Show.Title
		if show.Show.IDs.TMDB == 0 {
			libraryLog.Warningf("Missing TMDB ID for %s", title)
			continue
		}
		tmdbId := strconv.Itoa(show.Show.IDs.TMDB)
		if updating == "false" && config.Get().IgnoreDuplicates == true {
			if inJsonDb, err := InJsonDB(tmdbId, LShow); err != nil || inJsonDb == true {
				libraryLog.Warningf("%s already in library", title)
				continue
			}
			showId := tmdbId
			if config.Get().TvScraper == TVDBScraper {
					showId = strconv.Itoa(show.Show.IDs.TVDB)
			}
			if isDuplicateShow(showId, libraryShows) {
				libraryLog.Warningf("%s (%s) already in library", title, showId)
				continue
			}
		}
		if err := WriteShowStrm(tmdbId, ShowsLibraryPath); err != nil {
			libraryLog.Errorf("Unable to write strm file for %s", title)
			continue
		}
		if err := UpdateJsonDB(DBPath, tmdbId, LShow); err != nil {
			libraryLog.Error("Unable to update json DB")
			continue
		}
	}

	if updating == "false" {
		xbmc.Notify("Quasar", "LOCALIZE[30221]", config.AddonIcon())
	}
	libraryLog.Notice("Show watchlist added")
	ctx.String(200, "")
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
			if episode == nil {
				continue
			}
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

	tmdbId := ctx.Params.ByName("tmdbId")
	Id, _ := strconv.Atoi(tmdbId)
	show := tmdb.GetShow(Id, "en")

	if show == nil {
		ctx.String(200, "Unable to find show to remove")
		return
	}

	ShowStrm := toFileName(fmt.Sprintf("%s (%s)", show.Name, strings.Split(show.FirstAirDate, "-")[0]))
	ShowPath := filepath.Join(ShowsLibraryPath, ShowStrm)

	if err := RemoveFromJsonDB(DBPath, tmdbId, LShow); err != nil {
		ctx.String(200, "Unable to remove show from db")
		return
	}

	if err := os.RemoveAll(ShowPath); err != nil {
		ctx.String(200, "Unable to remove show folder")
		return
	}

	xbmc.Notify("Quasar", "LOCALIZE[30222]", config.AddonIcon())
	libraryLog.Notice("Show removed")
	ctx.String(200, "")
	xbmc.Refresh()
}
