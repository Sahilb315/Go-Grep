package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/spf13/cobra"
)

const (
	Red   = "\033[31m"
	Reset = "\033[0m"
)

func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "go-grep",
		Short: "Grep in go",
	}

	findCmd := &cobra.Command{
		Use:   "search [term] [file...]",
		Short: "Search for a word in input or files",
		Args:  cobra.MinimumNArgs(1),
		RunE:  runSearch,
	}

	findCmd.Flags().StringSliceP("file", "f", nil, "Files to search in (can specify multiple)")
	findCmd.Flags().BoolP("case-sensitive", "c", false, "Enable case-sensitive search")
	rootCmd.AddCommand(findCmd)

	return rootCmd
}

func Execute() {
	if err := NewRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func runSearch(cmd *cobra.Command, args []string) error {
	files, err := cmd.Flags().GetStringSlice("file")
	if err != nil {
		return fmt.Errorf("error getting file flag: %w", err)
	}

	caseSensitive, err := cmd.Flags().GetBool("case-sensitive")
	if err != nil {
		return fmt.Errorf("error getting case-sensitive flag: %w", err)
	}

	searchTerm := args[0]
	if !caseSensitive {
		searchTerm = strings.ToLower(searchTerm)
	}

	if len(files) == 0 && len(args) > 1 {
		files = args[1:]
	}

	if len(files) == 0 {
		return searchStdin(searchTerm, caseSensitive)
	}

	return searchFiles(files, searchTerm, caseSensitive)
}

func searchStdin(searchTerm string, caseSensitive bool) error {
	return processReader(os.Stdin, "stdin", searchTerm, caseSensitive)
}

func searchFiles(files []string, searchTerm string, caseSensitive bool) error {
	var wg sync.WaitGroup
	results := make(chan string)
	errors := make(chan error)

	for _, fileName := range files {
		wg.Add(1)
		go func(file string) {
			defer wg.Done()
			f, err := os.Open(file)
			if err != nil {
				errors <- fmt.Errorf("error opening %s: %w", file, err)
				return
			}
			defer f.Close()

			if err := processReader(f, file, searchTerm, caseSensitive); err != nil {
				errors <- err
			}
		}(fileName)
	}

	go func() {
		wg.Wait()
		close(results)
		close(errors)
	}()

	for err := range errors {
		fmt.Fprintln(os.Stderr, err)
	}

	return nil
}

func processReader(r io.Reader, source string, searchTerm string, caseSensitive bool) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	for scanner.Scan() {
		if line := highlightMatches(scanner.Text(), searchTerm, caseSensitive); line != "" {
			fmt.Printf("%s: %s\n", source, line)
		}
	}

	return scanner.Err()
}

func highlightMatches(line, searchTerm string, caseSensitive bool) string {
	if !caseSensitive {
		line = strings.ToLower(line)
	}

	if !strings.Contains(line, searchTerm) {
		return ""
	}

	var result strings.Builder
	lastIndex := 0

	for {
		index := strings.Index(line[lastIndex:], searchTerm)
		if index == -1 {
			result.WriteString(line[lastIndex:])
			break
		}

		index += lastIndex
		result.WriteString(line[lastIndex:index])
		result.WriteString(Red)
		result.WriteString(line[index : index+len(searchTerm)])
		result.WriteString(Reset)
		lastIndex = index + len(searchTerm)
	}

	return result.String()
}
