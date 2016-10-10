package main

import (
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/Luzifer/rconfig"
	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox"
	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox/files"
)

var (
	cfg = struct {
		DropboxToken   string `flag:"dropbox-token" env:"DROPBOX_TOKEN" description:"Dropbox token to use to access API"`
		ForceOverwrite bool   `flag:"force-overwrite" default:"false" description:"Upload files even when they exist on target"`
		Verbose        bool   `flag:"verbose,v" default:"false" description:"Enable verbose logging"`
		VersionAndExit bool   `flag:"version" default:"false" description:"Prints current version and exits"`
	}{}

	version = "dev"
)

func init() {
	if err := rconfig.Parse(&cfg); err != nil {
		log.Fatalf("Unable to parse commandline options: %s", err)
	}

	if cfg.VersionAndExit {
		fmt.Printf("dbx-sync %s\n", version)
		os.Exit(0)
	}
}

func main() {
	if len(rconfig.Args()) != 3 {
		log.Fatalf("Usage: dbx-sync <source path> <destination prefix>")
	}

	sourcePath := rconfig.Args()[1]
	destPrefix := rconfig.Args()[2]

	if f, err := os.Stat(sourcePath); err != nil || !f.IsDir() {
		log.Fatalf("Source path %q does not exist or is not a directory.", sourcePath)
	}

	if destPrefix[0] != '/' {
		log.Fatalf("Destination prefix must start with '/': %q", destPrefix)
	}

	config := dropbox.Config{Token: cfg.DropboxToken, Verbose: cfg.Verbose}
	dbx := files.New(config)

	remoteFilesPresent, err := getRemoteFileList(dbx, rconfig.Args()[2])
	if err != nil {
		log.Fatalf("Error while doing getRemoteFileList request: %s", err)
	}

	localFilesPresent, err := getLocalFileList(rconfig.Args()[1])
	if err != nil {
		log.Fatalf("Error while reading local file list: %s", err)
	}

	for f, lmt := range localFilesPresent {
		dest := path.Join(destPrefix, strings.Replace(f, sourcePath, "", -1))
		if rmt, ok := remoteFilesPresent[dest]; ok && !cfg.ForceOverwrite && !rmt.Before(lmt) {
			if cfg.Verbose {
				log.Printf("Remote %q already there, not uploading.", dest)
			}
			continue
		}

		log.Printf("Uploading %q to %q", f, dest)
		if err := uploadFile(dbx, f, dest); err != nil {
			log.Fatalf("Error while uploading %q: %s", dest, err)
		}
	}

}

func uploadFile(dbx files.Client, sourceFile, destinationFile string) error {
	info, err := os.Stat(sourceFile)
	if err != nil {
		return err
	}

	content, err := os.Open(sourceFile)
	if err != nil {
		return err
	}
	defer content.Close()

	modTime := time.Unix(info.ModTime().Unix(), 0)

	writeMode := &files.WriteMode{}
	writeMode.Tag = files.WriteModeOverwrite

	_, err = dbx.Upload(&files.CommitInfo{
		Path:           destinationFile,
		Mode:           writeMode,
		ClientModified: modTime,
	}, content)
	return err
}

func getRemoteFileList(dbx files.Client, prefix string) (map[string]time.Time, error) {
	remoteFilesPresent := map[string]time.Time{}

	lfr, err := dbx.ListFolder(&files.ListFolderArg{
		Path:      prefix,
		Recursive: true,
	})
	if err != nil {
		if strings.Contains(err.Error(), "path/not_found/") {
			return remoteFilesPresent, nil
		}
		return remoteFilesPresent, err
	}
	for {
		for _, f := range lfr.Entries {
			if fm, ok := f.(*files.FileMetadata); ok {
				remoteFilesPresent[fm.PathDisplay] = fm.ClientModified
			}
		}

		if !lfr.HasMore {
			break
		}

		lfr, err = dbx.ListFolderContinue(&files.ListFolderContinueArg{Cursor: lfr.Cursor})
		if err != nil {
			return remoteFilesPresent, err
		}
	}

	return remoteFilesPresent, nil
}

func getLocalFileList(prefix string) (map[string]time.Time, error) {
	fileList := map[string]time.Time{}

	return fileList, filepath.Walk(prefix, func(path string, f os.FileInfo, err error) error {
		if !f.IsDir() {
			fileList[path] = f.ModTime()
		}
		return nil
	})
}
