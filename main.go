/**
This is a pretty simple photo/large file deduplication program. It compares files by first filesize and does a secondary sweep
by comparing the SHA1 sum of the files. By default it will not do any actions unless the -dryrun flag is set to true.
At that point it will move the duplicates into a _Rejected subfolder next to the original file (same pattern as for the
https://www.fastrawviewer.com/ program. That folder can be cleaned either manually or by using find.

find . -type f -path '*_Rejected/*' -print -delete
find . -type d -name '_Rejected' -empty -delete

The original is found by just grabbing the shortest path among all duplicates. Since I am organising photos in
yyyy-mm-dd format with `exiftool` it doesnt that much which original I keep.
*/
package main

import (
	"crypto/sha1"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Where duplicates will be moved
const rejectFolder = "_Rejected"

// These are the only file suffixes that this program will check
var validExt = []string{
	".jpg",
	".jpeg",
	".mov",
	".nef",
	".raf",
	".mp4",
	".png",
	".tiff",
	".heic",
	".dng",
	".raf",
	".tiff",
	".png",
	".mp4",
	".mkv",
	".tgz",
	".zip",
	".rar",
}

type Hash [20]byte

func main() {
	var dryRun = true
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: %s path\n\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.BoolVar(&dryRun, "dryrun", true, "Will not move duplicate files if set to true (default)")
	flag.Parse()
	path := flag.Arg(0)

	if path == "" {
		flag.Usage()
		os.Exit(1)
	}

	fmt.Printf("Scanning directory and comparing file sizes\n")

	fileSizes := make(map[int64][]string)
	printer := &ProgressPrinter{}

	var permissionErrors []error

	err := filepath.Walk(path, func(path string, info os.FileInfo, inErr error) error {
		if inErr != nil {
			permissionErrors = append(permissionErrors, inErr)
			printer.Err()
			return nil
		}

		if strings.Contains(path, rejectFolder) {
			return nil
		}

		if !info.Mode().IsRegular() {
			return nil
		}

		for _, validExt := range validExt {
			if strings.ToLower(filepath.Ext(path)) == validExt {
				fileSizes[info.Size()] = append(fileSizes[info.Size()], path)
				printer.Print(len(fileSizes[info.Size()]) > 1)
				return nil
			}
		}
		return nil
	})

	handleError(err)
	fmt.Printf("\n\n")

	if len(permissionErrors) > 0 {
		fmt.Print("The following errors were encountered during the scan:\n\n")
		for _, err := range permissionErrors {
			fmt.Printf(" - '%s'\n", err)
		}
		fmt.Print("\n")
	}

	candidates := duplicatesInt64(fileSizes)

	fmt.Printf("Comparing %d out of %d files in more detail\n", len(candidates), len(fileSizes))

	fileHashes := make(map[Hash][]string)
	printer = &ProgressPrinter{Total: len(candidates)}
	for _, filePath := range candidates {
		sum, err := fileSHA1Sum(filePath)
		handleError(err)
		fileHashes[sum] = append(fileHashes[sum], filePath)
		printer.Print(len(fileHashes[sum]) > 1)
	}
	fmt.Printf("\n\n")

	if dryRun {
		fmt.Println("Showing duplicates")
	} else {
		fmt.Printf("Moving duplicates into %s folders\n", rejectFolder)
	}

	duplicates := duplicatesSHA1(fileHashes)
	sort.Sort(ByShortest(duplicates))

	for _, paths := range duplicates {
		i := shortestIdx(paths)
		original := paths[i]
		paths = append(paths[:i], paths[i+1:]...)

		rejectedDir := filepath.Join(filepath.Dir(original), rejectFolder)
		if _, err := os.Stat(rejectedDir); !dryRun && os.IsNotExist(err) {
			err := os.Mkdir(rejectedDir, 0755)
			handleError(err)
		}

		fmt.Printf("\n%s\n", original)
		for i, f := range paths {
			if dryRun {
				fmt.Println(f)
				continue
			}
			newLocation := copyPath(original, rejectedDir, i+1)
			fmt.Println(newLocation)
			err := os.Rename(f, newLocation)
			handleError(err)
		}
	}
}

func handleError(err error) {
	if err == nil {
		return
	}
	fmt.Printf("Error: '%s'\n", err)
	os.Exit(1)
}

type ByShortest [][]string

func (s ByShortest) Len() int { return len(s) }

func (s ByShortest) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

func (s ByShortest) Less(i, j int) bool {
	a := shortestIdx(s[i])
	b := shortestIdx(s[j])
	return strings.ToLower(s[i][a]) < strings.ToLower(s[j][b])
}

func shortestIdx(a []string) int {
	idx := 0
	for i, path := range a {
		if len(path) < len(a[idx]) {
			idx = i
		}
	}
	return idx
}

func copyPath(filePath, dest string, number int) string {
	ext := filepath.Ext(filePath)
	name := filePath[0 : len(filePath)-len(ext)]
	copyName := fmt.Sprintf("%s_%d%s", filepath.Base(name), number, ext)
	return filepath.Join(dest, copyName)
}

func fileSHA1Sum(filePath string) (Hash, error) {
	hasher := sha1.New()
	var hashInBytes Hash

	file, err := os.Open(filePath)
	if err != nil {
		return hashInBytes, err
	}
	defer file.Close()

	defer hasher.Reset()
	if _, err := io.Copy(hasher, file); err != nil {
		return hashInBytes, err
	}

	copy(hashInBytes[:], hasher.Sum(nil))
	return hashInBytes, nil
}

func duplicatesInt64(f map[int64][]string) []string {
	var result []string
	for _, paths := range f {
		if len(paths) < 2 {
			continue
		}
		result = append(result, paths...)
	}
	return result
}

func duplicatesSHA1(f map[Hash][]string) [][]string {
	var result [][]string
	for _, paths := range f {
		if len(paths) < 2 {
			continue
		}
		result = append(result, paths)
	}
	return result
}

// ProgressPrinter will print a progress counter and if Total is set a percentage of how far the along the work has gone
type ProgressPrinter struct {
	Total int // the total number of entries that will be printed, zero if unknown

	current   int
	lineCount int
}

func (p *ProgressPrinter) Err() {
	p.inc()
	fmt.Print("e")
}

func (p *ProgressPrinter) Print(dupe bool) {
	p.inc()
	if dupe {
		fmt.Print("d")
	} else {
		fmt.Print(".")
	}
}

func (p *ProgressPrinter) inc() {
	if p.lineCount == 77 || p.lineCount == 0 {
		if p.Total == 0 {
			fmt.Printf("\n   ")
		} else {
			fmt.Printf("\n%2.0f%% ", float32(p.current)/float32(p.Total)*100)
		}
		p.lineCount = 0
	}
	p.current++
	p.lineCount++
}
