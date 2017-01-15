package osdb

import (
	"io"
	"os"
	"fmt"
	"strconv"
	"strings"

	"github.com/kolo/xmlrpc"
	"github.com/op/go-logging"
)

const (
	DefaultOSDBServer = "https://api.opensubtitles.org/xml-rpc"
	DefaultUserAgent  = "XBMC_Subtitles_Login_v5.0.16" // XBMC OpenSubtitles Agent
	SearchLimit       = 100
	StatusSuccess     = "200 OK"
)

var log = logging.MustGetLogger("osdb")

type Client struct {
	UserAgent string
	Token     string
	Login     string
	Password  string
	Language  string
	*xmlrpc.Client
}

type Movie struct {
	Id             string            `xmlrpc:"id"`
	Title          string            `xmlrpc:"title"`
	Cover          string            `xmlrpc:"cover"`
	Year           string            `xmlrpc:"year"`
	Duration       string            `xmlrpc:"duration"`
	TagLine        string            `xmlrpc:"tagline"`
	Plot           string            `xmlrpc:"plot"`
	Goofs          string            `xmlrpc:"goofs"`
	Trivia         string            `xmlrpc:"trivia"`
	Cast           map[string]string `xmlrpc:"cast"`
	Directors      map[string]string `xmlrpc:"directors"`
	Writers        map[string]string `xmlrpc:"writers"`
	Awards         string            `xmlrpc:"awards"`
	Genres         []string          `xmlrpc:"genres"`
	Countries      []string          `xmlrpc:"country"`
	Languages      []string          `xmlrpc:"language"`
	Certifications []string          `xmlrpc:"certification"`
}

type SearchPayload struct {
	Query     string `xmlrpc:"query"`
	Hash      string `xmlrpc:"moviehash"`
	Size      int64  `xmlrpc:"moviebytesize"`
	IMDBId    string `xmlrpc:"imdbid"`
	Languages string `xmlrpc:"sublanguageid"`
}

// A collection of movies.
type Movies []Movie

func (m Movies) Empty() bool {
	return len(m) == 0
}

func NewClient() (*Client, error) {
	rpc, err := xmlrpc.NewClient(DefaultOSDBServer, nil)
	if err != nil {
		return nil, err
	}

	c := &Client{
		UserAgent: DefaultUserAgent,
		Client:    rpc, // xmlrpc.Client
	}

	return c, nil
}

func (c *Client) SearchSubtitles(payloads []SearchPayload) (Subtitles, error) {
	res := struct {
		Data Subtitles `xmlrpc:"data"`
	}{}

	args := []interface{}{
		c.Token,
		payloads,
	}
	if err := c.Call("SearchSubtitles", args, &res); err != nil {
		log.Error(err)
		if !strings.Contains(err.Error(), "type mismatch") {
			return nil, err
		}
	}
	return res.Data, nil
}

// Search movies on IMDB.
func (c *Client) SearchOnImdb(q string) (Movies, error) {
	params := []interface{}{c.Token, q}
	res := struct {
		Status string `xmlrpc:"status"`
		Data   Movies `xmlrpc:"data"`
	}{}
	if err := c.Call("SearchMoviesOnIMDB", params, &res); err != nil {
		return nil, err
	}
	if res.Status != StatusSuccess {
		return nil, fmt.Errorf("SearchMoviesOnIMDB error: %s", res.Status)
	}
	return res.Data, nil
}

// Get movie details from IMDB.
func (c *Client) GetImdbMovieDetails(id string) (*Movie, error) {
	params := []interface{}{c.Token, id}
	res := struct {
		Status string `xmlrpc:"status"`
		Data   Movie  `xmlrpc:"data"`
	}{}
	if err := c.Call("GetIMDBMovieDetails", params, &res); err != nil {
		return nil, err
	}
	if res.Status != StatusSuccess {
		return nil, fmt.Errorf("GetIMDBMovieDetails error: %s", res.Status)
	}
	return &res.Data, nil
}

// Download subtitles by file ID.
func (c *Client) DownloadSubtitles(ids []int) ([]SubtitleFile, error) {
	params := []interface{}{c.Token, ids}
	res := struct {
		Status string         `xmlrpc:"status"`
		Data   []SubtitleFile `xmlrpc:"data"`
	}{}
	if err := c.Call("DownloadSubtitles", params, &res); err != nil {
		return nil, err
	}
	if res.Status != StatusSuccess {
		return nil, fmt.Errorf("DownloadSubtitles error: %s", res.Status)
	}
	return res.Data, nil
}

// Save subtitle file to disk, using the OSDB specified name.
func (c *Client) Download(s *Subtitle) error {
	return c.DownloadTo(s, s.SubFileName)
}

// Save subtitle file to disk, using the specified path.
func (c *Client) DownloadTo(s *Subtitle, path string) (err error) {
	id, err := strconv.Atoi(s.IDSubtitleFile)
	if err != nil {
		return
	}

	// Download
	files, err := c.DownloadSubtitles([]int{id})
	if err != nil {
		return
	}
	if len(files) == 0 {
		return fmt.Errorf("No file match this subtitle ID")
	}

	// Save to disk.
	r, err := files[0].Reader()
	if err != nil {
		return
	}
	defer r.Close()

	w, err := os.Create(path)
	if err != nil {
		return
	}
	defer w.Close()

	_, err = io.Copy(w, r)
	return
}

// Checks whether OSDB already has subtitles for a movie and subtitle
// files.
func (c *Client) HasSubtitlesForFiles(movie_file string, sub_file string) (bool, error) {
	subtitle, err := NewSubtitleWithFile(movie_file, sub_file)
	if err != nil {
		return true, err
	}
	return c.HasSubtitles(Subtitles{subtitle})
}

// Checks whether subtitles already exists in OSDB. The mandatory fields in the
// received Subtitle slice are: SubHash, SubFileName, MovieHash, MovieByteSize,
// and MovieFileName.
func (c *Client) HasSubtitles(subs Subtitles) (bool, error) {
	subArgs, err := subs.toUploadParams()
	if err != nil {
		return true, err
	}
	args := []interface{}{c.Token, subArgs}
	res := struct {
		Status string `xmlrpc:"status"`
		Exists int    `xmlrpc:"alreadyindb"`
	}{}
	if err := c.Call("TryUploadSubtitles", args, &res); err != nil {
		return true, err
	}
	if res.Status != StatusSuccess {
		return true, fmt.Errorf("HasSubtitles: %s", res.Status)
	}

	return res.Exists == 1, nil
}

// Keep session alive
func (c *Client) Noop() (err error) {
	res := struct {
		Status string `xmlrpc:"status"`
	}{}
	err = c.Call("NoOperation", []interface{}{c.Token}, &res)
	if err == nil && res.Status != StatusSuccess {
		err = fmt.Errorf("NoOp: %s", res.Status)
	}
	return
}

// Login to the API, and return a session token.
func (c *Client) LogIn(user string, pass string, lang string) (err error) {
	c.Login = user
	c.Password = pass
	c.Language = lang
	args := []interface{}{user, pass, lang, c.UserAgent}
	res := struct {
		Status string `xmlrpc:"status"`
		Token  string `xmlrpc:"token"`
	}{}
	if err = c.Call("LogIn", args, &res); err != nil {
		return
	}

	if res.Status != StatusSuccess {
		return fmt.Errorf("Login: %s", res.Status)
	}
	c.Token = res.Token
	return
}

// Logout...
func (c *Client) LogOut() (err error) {
	args := []interface{}{c.Token}
	res := struct {
		Status string `xmlrpc:"status"`
	}{}
	return c.Call("LogOut", args, &res)
}

// Build query parameters for hash-based movie search.
func (c *Client) fileToParams(path string, langs []string) (*[]interface{}, error) {
	// File size
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	fi, err := file.Stat()
	if err != nil {
		return nil, err
	}
	size := fi.Size()

	// File hash
	h, err := HashFile(file)
	if err != nil {
		return nil, err
	}

	params := []interface{}{
		c.Token,
		[]struct {
			Hash  string `xmlrpc:"moviehash"`
			Size  int64  `xmlrpc:"moviebytesize"`
			Langs string `xmlrpc:"sublanguageid"`
		}{{
			h,
			size,
			strings.Join(langs, ","),
		}},
	}
	return &params, nil
}
