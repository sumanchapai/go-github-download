// package downreleasegithub
package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strings"
	"time"
)

type Repo struct {
	owner string
	name  string
}

func (r Repo) String() string {
	return fmt.Sprintf("%v/%v", r.owner, r.name)
}

type DownloadObject struct {
	repo   Repo
	binary string // Name of the binary
}

type ReleasesJSONResonse struct {
	ID                      int    `json:"id"`
	TagName                 string `json:"tag_name"`
	UpdateURL               string `json:"update_url"`
	UpdateAuthenticityToken string `json:"update_authenticity_token"`
	DeleteURL               string `json:"delete_url"`
	DeleteAuthenticityToken string `json:"delete_authenticity_token"`
	EditURL                 string `json:"edit_url"`
}

// Here we're creating a custom client with timeout because the default
// http client doesn't time out which could be problematic see here:
// https://medium.com/@nate510/don-t-use-go-s-default-http-client-4804cb19f779
var netClient = &http.Client{Timeout: time.Second * 10}

// Get the download link for given repo for the system on which command is being run
// returns non-nil error if ecountered erros
func LatestDownloadLink(downloadable DownloadObject, version string) string {
	// Get the filename of the downloadable
	versionStringVTrimmed := strings.TrimPrefix(version, "v")
	filename := fmt.Sprintf("%v_%v_%v_%v.tar.gz", downloadable.binary, versionStringVTrimmed, runtime.GOOS, runtime.GOARCH)
	// "https://github.com/Guerrilla-Interactive/ngo/releases/download/v1.1.0/ngo_1.1.0_linux_arm64.tar.gz"
	return fmt.Sprintf("https://github.com/%v/%v/releases/download/%v/%v",
		downloadable.repo.owner, downloadable.repo.name, version, filename)
}

// Return the latest release version tag
// error returned is non nil if no release found
func GetLatestVersionString(repo Repo) (string, error) {
	latestRelease := fmt.Sprintf("https://github.com/%v/%v/releases/latest", repo.owner, repo.name)
	latestReleaseUrl, err := url.Parse(latestRelease)
	if err != nil {
		return "", err
	}
	req := http.Request{
		Method: "GET",
		URL:    latestReleaseUrl,
		Header: map[string][]string{
			"Accept": {"application/json"},
		},
	}
	resp, err := netClient.Do(&req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	buf, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var releasesResponse ReleasesJSONResonse
	json.Unmarshal(buf, &releasesResponse)
	return releasesResponse.TagName, nil
}

func main() {
	repos := []Repo{
		{owner: "Guerrilla-Interactive", name: "ngo"},
	}
	for _, repo := range repos {
		version, err := GetLatestVersionString(repo)
		if err != nil {
			log.Fatal(err)
		}
		downloadLink := LatestDownloadLink(DownloadObject{repo: repo, binary: repo.name}, version)
		resp, err := netClient.Get(downloadLink)
		if err != nil {
			log.Fatal(err)
		}
		ExtractTarGz(resp.Body)
		file := repo.name
		// Change permission of the downloaded file to make it executable
		if err := os.Chmod(file, 0o755); err != nil {
			log.Fatal(err)
		}
		// Based on OS, move the file to appropriate directory
		// and append the PATH environment variable
		fmt.Printf("%v\n%v\n", repo, downloadLink)
	}
}

// Taken from
// https://gist.github.com/indraniel/1a91458984179ab4cf80?permalink_comment_id=2122149#gistcomment-2122149
func ExtractTarGz(gzipStream io.Reader) {
	uncompressedStream, err := gzip.NewReader(gzipStream)
	if err != nil {
		log.Fatal("ExtractTarGz: NewReader failed")
	}

	tarReader := tar.NewReader(uncompressedStream)

	for {
		header, err := tarReader.Next()

		if err == io.EOF {
			break
		}

		if err != nil {
			log.Fatalf("ExtractTarGz: Next() failed: %s", err.Error())
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.Mkdir(header.Name, 0o755); err != nil {
				log.Fatalf("ExtractTarGz: Mkdir() failed: %s", err.Error())
			}
		case tar.TypeReg:
			outFile, err := os.Create(header.Name)
			if err != nil {
				log.Fatalf("ExtractTarGz: Create() failed: %s", err.Error())
			}
			defer outFile.Close()
			if _, err := io.Copy(outFile, tarReader); err != nil {
				log.Fatalf("ExtractTarGz: Copy() failed: %s", err.Error())
			}
		default:
			log.Fatalf(
				"ExtractTarGz: uknown type: %v in %v",
				header.Typeflag,
				header.Name)
		}
	}
}
