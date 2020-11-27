package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

func main() {
	var ref string
	var url string
	var folder string
	var dest string
	flag.StringVar(&ref, "ref", "master", "ref to fetch master or pull/1/head")
	flag.StringVar(&url, "repo", "", "in the format: git@github.com:owner/repo.git")
	flag.StringVar(&folder, "folder", "kubernetes", "folder to download")
	flag.StringVar(&dest, "dest", "/tmp/dl", "folder to download files into")

	flag.Parse()
	if url == "" {
		flag.PrintDefaults()
		os.Exit(1)
	}
	token := os.Getenv("GITHUB_TOKEN")
	owner, repo := ownerAndRepo(url)
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)

	if ref != "master" {
		if strings.Contains(ref, "pull") {
			log.Printf("Detected PR %v", ref)
			x := strings.Split(ref, "/")[1]
			prnumber, err := strconv.Atoi(x)
			if err != nil {
				panic(err)
			}
			pr, _, err := client.PullRequests.Get(ctx, owner, repo, prnumber)
			if err == nil {
				ref = pr.GetHead().GetSHA()
				fmt.Println("Ref is ", ref)
			} else {
				panic(err)
			}
		}
	}
	start := time.Now()
	var wg sync.WaitGroup
	ch := make(chan file)
	go func(ch chan file) {
		for {
			v, ok := <-ch
			if ok == false {
				break
			}
			wg.Add(1)

			//fmt.Println("Downloading ", v, ok)

			go func(v file) {
				resp, err := http.Get(v.url)
				if err != nil {
					log.Printf("error get content: %v", err)
					os.Exit(1)
				}
				defer resp.Body.Close()
				out, err := os.Create(v.file)
				if err != nil {
					log.Println("os.Create: ", err)
					os.Exit(1)
				}
				defer out.Close()
				_, err = io.Copy(out, resp.Body)
				if err != nil {
					log.Println(err)
				}
			}(v)
			wg.Done()
		}
	}(ch)
	var ss sync.WaitGroup
	ss.Add(1)
	r := Repo{org: owner, name: repo, ref: ref}
	r.Get(folder, client, ch, &ss, dest)
	fmt.Printf("Done downloading %v in %.3v secs\n", repo, (time.Now().Sub(start)))
	ss.Wait()
	close(ch)
	wg.Wait()
}

type file struct {
	file string
	url  string
}

type Repo struct {
	org  string
	name string
	ref  string
}

func (r *Repo) Get(path string, client *github.Client, ch chan file, wg *sync.WaitGroup, dest string) {
	defer wg.Done()
	_, dircontent, _, err := client.Repositories.GetContents(context.Background(), r.org, r.name, path, &github.RepositoryContentGetOptions{Ref: r.ref})
	if err != nil {
		log.Printf("Repositories.DownloadContents returned error: %v", err)
	}
	var ss sync.WaitGroup

	for _, v := range dircontent {
		if v.GetType() == "dir" {
			err := os.MkdirAll(dest+v.GetPath(), 0755)
			if err != nil {
				log.Printf("error making dir: %v", err)
			}
			ss.Add(1)
			go r.Get(v.GetPath(), client, ch, &ss, dest)
		} else {
			ch <- file{file: dest + v.GetPath(), url: v.GetDownloadURL()}

		}
	}
	ss.Wait()

}

func ownerAndRepo(url string) (string, string) {
	x := strings.Split(url, ":")[1]
	y := strings.Split(x, "/")
	owner := y[0]
	repo := y[1]
	repo = strings.Replace(repo, ".git", "", -1)
	return owner, repo
}
