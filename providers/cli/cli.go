package cli

import (
	"encoding/json"
	"log"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/steeve/pulsar/providers"
	"github.com/steeve/pulsar/tmdb"
	"github.com/steeve/pulsar/trakt"
)

const (
	DefaultCLITimeout = 5 * time.Second
)

type CLISearcher struct {
	providers.MovieSearcher
	providers.EpisodeSearcher

	cmdLine []string
}

type SearchPayload struct {
	Method  string `json:"method"`
	Query   string `json:"query,omitempty"`
	Name    string `json:"name,omitempty"`
	IMDBId  string `json:"imdb_id,omitempty"`
	Year    int    `json:"year,omitempty"`
	Season  int    `json:"season,omitempty"`
	Episode int    `json:"episode,omitempty"`
}

func NewCLISearcher(cmdLine []string) *CLISearcher {
	return &CLISearcher{
		cmdLine: cmdLine,
	}
}

func (searcher *CLISearcher) runCmdLine(payload *SearchPayload) (links []string) {
	cmd := exec.Command(searcher.cmdLine[0], searcher.cmdLine[1:]...)

	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()

	cmd.Start()

	done := make(chan error, 1)
	go func() {
		json.NewEncoder(stdin).Encode(payload)
		stdin.Close()
		json.NewDecoder(stdout).Decode(&links)
		done <- cmd.Wait()
	}()

	select {
	case <-time.After(DefaultCLITimeout):
		if err := cmd.Process.Kill(); err != nil {
			log.Println("Failed to kill: ", err)
		}
		log.Println("Process was killed:", cmd.Args)
	case err := <-done:
		if err != nil {
			log.Println("Process exited with error.", cmd.Args, err)
		}
		break
	}

	return
}

func (searcher *CLISearcher) SearchMovieLinks(movie *tmdb.Movie) []string {
	yearStr := strings.Split(movie.ReleaseDate, "-")[0]
	year, _ := strconv.Atoi(yearStr)
	payload := &SearchPayload{
		Method: "search_movie",
		Name:   movie.Title,
		IMDBId: movie.IMDBId,
		Year:   year,
	}
	return searcher.runCmdLine(payload)
}

func (searcher *CLISearcher) SearchEpisodeLinks(episode *trakt.ShowEpisode) []string {
	normalized_title := episode.Show.Title
	normalized_title = strings.ToLower(normalized_title)
	normalized_title = regexp.MustCompile(`(\d+|\W+|\s+)`).ReplaceAllString(normalized_title, " ")
	normalized_title = strings.TrimSpace(normalized_title)

	payload := &SearchPayload{
		Method:  "search_episode",
		Name:    normalized_title,
		Season:  episode.Season.Season,
		Episode: episode.Episode,
	}
	return searcher.runCmdLine(payload)
}
