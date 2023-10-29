package main

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"
)

type Client struct {
	owner    string
	repo     string
	basePath string
	branch   string
}

func main() {
	app := cli.App{
		Name:        "serve-git",
		Description: "serve files from git repository",
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:    "port",
				Aliases: []string{"p"},
				Usage:   "port to serve on",
				Value:   8080,
			},
			&cli.StringFlag{
				Name:     "repo",
				Usage:    "repository url",
				Required: true,
			},
			&cli.StringFlag{
				Name:  "branch",
				Value: "master",
			},
			&cli.StringFlag{
				Name:  "base",
				Usage: "base files dir",
				Value: "/",
			},
		},
		Action: func(ctx *cli.Context) error {
			port := ctx.Int("port")
			repo := ctx.String("repo")
			branch := ctx.String("branch")
			basePath := ctx.String("base")

			log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

			repoURL, err := url.Parse(repo)
			if err != nil {
				return fmt.Errorf("parse repo url %q: %w", repo, err)
			}

			// /{owner}/{repoo}
			owner, repoo := path.Split(repoURL.Path)

			client := Client{
				owner:    path.Base(owner),
				repo:     repoo,
				basePath: basePath,
				branch:   branch,
			}
			http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					log.Info().Str("method", r.Method).Msg("not allowed method")
					w.WriteHeader(http.StatusMethodNotAllowed)
					return
				}

				filePath := path.Join(client.basePath, r.URL.Path)

				l := log.With().Str("path", filePath).Logger()

				fileURL := url.URL{
					Scheme: "https",
					Host:   "raw.githubusercontent.com",
					Path:   path.Join(client.owner, client.repo, client.branch, filePath),
				}

				resp, err := http.Get(fileURL.String())
				if err != nil {
					l.Info().Err(err).Send()

					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write([]byte(fmt.Sprintf("get file %q: %s", filePath, err.Error())))
					return
				}
				defer resp.Body.Close()

				if resp.StatusCode == http.StatusNotFound {
					filePath = path.Join(filePath, "index.html")

					l = log.With().Str("path", filePath).Logger()

					fileURL := url.URL{
						Scheme: "https",
						Host:   "raw.githubusercontent.com",
						Path:   path.Join(client.owner, client.repo, client.branch, filePath),
					}

					resp, err = http.Get(fileURL.String())
					if err != nil {
						l.Info().Err(err).Send()

						w.WriteHeader(http.StatusInternalServerError)
						_, _ = w.Write([]byte(fmt.Sprintf("get file %q: %s", filePath, err.Error())))
						return
					}
					defer resp.Body.Close()
				}

				contentType := mime.TypeByExtension(path.Ext(filePath))

				l.Info().Str("content_type", contentType).Send()

				w.Header().Add("Content-Type", contentType)
				w.WriteHeader(http.StatusOK)
				_, _ = io.Copy(w, resp.Body)
			})

			addr := fmt.Sprintf("0.0.0.0:%d", port)

			log.Info().
				Str("repo", repo).
				Str("addr", addr).
				Msg("serving")
			return http.ListenAndServe(addr, http.DefaultServeMux)
		},
	}
	if err := app.Run(os.Args); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}
