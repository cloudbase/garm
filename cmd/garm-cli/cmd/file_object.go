// Copyright 2025 Cloudbase Solutions SRL
//
//	Licensed under the Apache License, Version 2.0 (the "License"); you may
//	not use this file except in compliance with the License. You may obtain
//	a copy of the License at
//
//	     http://www.apache.org/licenses/LICENSE-2.0
//
//	Unless required by applicable law or agreed to in writing, software
//	distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//	WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
//	License for the specific language governing permissions and limitations
//	under the License.
package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/http/httputil"
	"os"
	"strings"
	"sync"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/spf13/cobra"

	apiClientObject "github.com/cloudbase/garm/client/objects"
	"github.com/cloudbase/garm/cmd/garm-cli/common"
	"github.com/cloudbase/garm/params"
)

var (
	filePath           string
	fileName           string
	fileObjTags        string
	fileObjDescription string
	fileObjPage        int64
	fileObjPageSize    int64
	outObjectPath      string
	forceOverwrite     bool
	quietMode          bool
)

// progressReader wraps an io.Reader and reports progress
type progressReader struct {
	reader      io.Reader
	total       int64
	current     int64
	lastPrinted int
	mu          sync.Mutex
}

func newProgressReader(r io.Reader, total int64) *progressReader {
	return &progressReader{
		reader: r,
		total:  total,
	}
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	pr.mu.Lock()
	pr.current += int64(n)
	pr.mu.Unlock()
	pr.printProgress()
	return n, err
}

func (pr *progressReader) printProgress() {
	pr.mu.Lock()
	defer pr.mu.Unlock()

	if pr.total == 0 {
		return
	}

	percent := int(float64(pr.current) / float64(pr.total) * 100)

	// Only print every 5% or at 100%
	if percent != pr.lastPrinted && (percent%5 == 0 || percent == 100) {
		mb := float64(pr.current) / 1024 / 1024
		totalMB := float64(pr.total) / 1024 / 1024
		fmt.Printf("\rUploading: %d%% (%.2f MB / %.2f MB)", percent, mb, totalMB)
		if percent == 100 {
			fmt.Println() // New line at completion
		}
		pr.lastPrinted = percent
	}
}

// giteaCredentialsCmd represents the gitea credentials command
var fileObjectCmd = &cobra.Command{
	Use:   "object",
	Short: "Manage simple object storage",
	Long: `Manage simple object storage.

This command allows you to use GARM as a simple, private internal-use object storage
system streamed to and from the database using blob I/O. The primary goal of this is
to allow users to store provider binaries, agent binaries runner tools and any other
type of files needed for a functional GARM deployment.

It is not meant to be used to serve files outside of the needs of GARM.`,
	Run: nil,
}

var fileObjListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List file objects",
	RunE: func(_ *cobra.Command, _ []string) error {
		if needsInit {
			return errNeedsInitError
		}

		listReq := apiClientObject.NewListFileObjectsParams()
		listReq.Tags = &fileObjTags
		listReq.Page = &fileObjPage
		listReq.PageSize = &fileObjPageSize
		response, err := apiCli.Objects.ListFileObjects(listReq, authToken)
		if err != nil {
			return err
		}
		formatFileObjsList(response.Payload)
		return nil
	},
}

var fileObjDeleteCmd = &cobra.Command{
	Use:     "remove",
	Aliases: []string{"delete", "del", "rm"},
	Short:   "List file objects",
	Args:    cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		objID := args[0]

		delReq := apiClientObject.NewDeleteFileObjectParams().WithObjectID(objID)
		err := apiCli.Objects.DeleteFileObject(delReq, authToken)
		if err != nil {
			return err
		}

		return nil
	},
}

var fileObjShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show a file object",
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		objID := args[0]

		getReq := apiClientObject.NewGetFileObjectParams().WithObjectID(objID)
		resp, err := apiCli.Objects.GetFileObject(getReq, authToken)
		if err != nil {
			return err
		}
		formatOneObject(resp.Payload)
		return nil
	},
}

var fileObjUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update a file object",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		objID := args[0]

		hasChanges := false
		updateParams := params.UpdateFileObjectParams{}
		updateReq := apiClientObject.NewUpdateFileObjectParams().WithObjectID(objID)
		if cmd.Flags().Changed("name") {
			hasChanges = true
			updateParams.Name = &fileName
		}

		if cmd.Flags().Changed("tags") && fileObjTags != "" {
			hasChanges = true
			updateParams.Tags = strings.Split(fileObjTags, ",")
		}

		if cmd.Flags().Changed("description") {
			hasChanges = true
			updateParams.Description = &fileObjDescription
		}

		if !hasChanges {
			fmt.Println("no changes made")
			return nil
		}

		updateReq.Body = updateParams
		resp, err := apiCli.Objects.UpdateFileObject(updateReq, authToken)
		if err != nil {
			return err
		}
		formatOneObject(resp.Payload)
		return nil
	},
}

var fileObjCreateCmd = &cobra.Command{
	Use:     "create",
	Aliases: []string{"add", "upload"},
	Short:   "Upload a file to the server",
	RunE: func(_ *cobra.Command, _ []string) error {
		if needsInit {
			return errNeedsInitError
		}
		if filePath == "" {
			return fmt.Errorf("missing file path")
		}
		stat, err := os.Stat(filePath)
		if err != nil {
			return fmt.Errorf("failed to acces file: %w", err)
		}
		file, err := os.Open(filePath)
		if err != nil {
			return fmt.Errorf("failed to open file: %w", err)
		}
		defer file.Close()

		var tags []string
		if fileObjTags != "" {
			tags = strings.Split(fileObjTags, ",")
		}

		// Wrap file reader with progress tracking
		progressR := newProgressReader(file, stat.Size())

		// Create request with progress-tracked file stream
		req, err := rawHTTPClient.NewRequest(http.MethodPost, "/objects/", progressR)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("X-File-Name", fileName)
		if len(fileObjDescription) > 2<<12 {
			return fmt.Errorf("description is too large (max 8KB)")
		}
		if fileObjDescription != "" {
			req.Header.Set("X-File-Description", fileObjDescription)
		}
		if len(tags) > 0 {
			req.Header.Set("X-Tags", strings.Join(tags, ","))
		}
		req.ContentLength = stat.Size()

		// Debug: dump request
		if debug {
			// Don't dump body for large uploads
			b, err2 := httputil.DumpRequestOut(req, false)
			if err2 != nil {
				return fmt.Errorf("failed to dump request: %w", err2)
			}
			fmt.Fprintf(os.Stderr, "DEBUG REQUEST:\n%s\n", string(b))
		}

		// Show initial progress
		fmt.Printf("Uploading %s (%.2f MB)...\n", fileName, float64(stat.Size())/1024/1024)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			if debug {
				fmt.Fprintf(os.Stderr, "DEBUG ERROR: %v\n", err)
			}
			return fmt.Errorf("failed to upload: %w", err)
		}
		defer resp.Body.Close()

		// Debug: dump response
		if debug {
			b, err2 := httputil.DumpResponse(resp, true)
			if err2 != nil {
				return fmt.Errorf("failed to dump response: %w", err2)
			}
			fmt.Fprintf(os.Stderr, "DEBUG RESPONSE:\n%s\n", string(b))
		}

		data, err := io.ReadAll(resp.Body)
		if err != nil {
			if debug {
				fmt.Fprintf(os.Stderr, "DEBUG ERROR reading response body: %v\n", err)
			}
			return fmt.Errorf("failed to read response body: %w", err)
		}

		// Check for non-2xx status codes
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			if debug {
				fmt.Fprintf(os.Stderr, "DEBUG ERROR: HTTP %d: %s\n", resp.StatusCode, string(data))
			}
			return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(data))
		}

		var fileResp params.FileObject
		if err := json.Unmarshal(data, &fileResp); err != nil {
			if debug {
				fmt.Fprintf(os.Stderr, "DEBUG ERROR decoding response: %v\nResponse body: %s\n", err, string(data))
			}
			return fmt.Errorf("failed to decode response: %w", err)
		}
		formatOneObject(fileResp)
		return nil
	},
}

var fileObjDownloadCmd = &cobra.Command{
	Use:          "download",
	Short:        "Download a file from the server",
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE: func(_ *cobra.Command, args []string) error {
		if needsInit {
			return errNeedsInitError
		}

		objectID := args[0]

		// Create request for download
		req, err := rawHTTPClient.NewRequest(http.MethodGet, fmt.Sprintf("/objects/%s/download", objectID), nil)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		// Debug: dump request
		if debug {
			b, err2 := httputil.DumpRequestOut(req, false)
			if err2 != nil {
				return fmt.Errorf("failed to dump request: %w", err2)
			}
			fmt.Fprintf(os.Stderr, "DEBUG REQUEST:\n%s\n", string(b))
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			if debug {
				fmt.Fprintf(os.Stderr, "DEBUG ERROR: %v\n", err)
			}
			return fmt.Errorf("failed to download: %w", err)
		}
		defer resp.Body.Close()

		// Debug: dump response headers (not body for large files)
		if debug {
			b, err2 := httputil.DumpResponse(resp, false)
			if err2 != nil {
				return fmt.Errorf("failed to dump response: %w", err2)
			}
			fmt.Fprintf(os.Stderr, "DEBUG RESPONSE:\n%s\n", string(b))
		}

		// Check for non-2xx status codes
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			data, _ := io.ReadAll(resp.Body)
			if debug {
				fmt.Fprintf(os.Stderr, "DEBUG ERROR: HTTP %d: %s\n", resp.StatusCode, string(data))
			}
			return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(data))
		}

		// Get filename from Content-Disposition header if not specified
		filename := outObjectPath
		if filename == "" {
			contentDisp := resp.Header.Get("Content-Disposition")
			if contentDisp != "" {
				// Parse Content-Disposition header
				_, params, err := mime.ParseMediaType(contentDisp)
				if err == nil && params["filename"] != "" {
					filename = params["filename"]
				}
			}
			if filename == "" {
				return fmt.Errorf("no output file specified and server did not provide filename")
			}
		}

		// Check if path exists
		if stat, err := os.Stat(filename); err == nil {
			if stat.IsDir() {
				return fmt.Errorf("output path is a directory: %s", filename)
			}
			if !forceOverwrite {
				return fmt.Errorf("file already exists: %s (use --force-overwrite to overwrite)", filename)
			}
		}

		// Create output file
		outFile, err := os.Create(filename)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer outFile.Close()

		// Get content length for progress tracking
		contentLength := resp.ContentLength
		if contentLength <= 0 {
			// Try to parse from header if not set
			contentLengthStr := resp.Header.Get("Content-Length")
			if contentLengthStr != "" {
				fmt.Sscanf(contentLengthStr, "%d", &contentLength)
			}
		}

		// Show initial progress
		if !quietMode {
			if contentLength > 0 {
				fmt.Printf("Downloading %s (%.2f MB)...\n", filename, float64(contentLength)/1024/1024)
			} else {
				fmt.Printf("Downloading %s...\n", filename)
			}
		}

		// Wrap reader with progress tracking
		var reader io.Reader = resp.Body
		if contentLength > 0 && !quietMode {
			progressR := newProgressReader(resp.Body, contentLength)
			// Change progress message for download
			reader = &downloadProgressReader{progressR}
		}

		// Copy to file
		written, err := io.Copy(outFile, reader)
		if err != nil {
			if debug {
				fmt.Fprintf(os.Stderr, "DEBUG ERROR writing to file: %v\n", err)
			}
			return fmt.Errorf("failed to write file: %w", err)
		}

		if !quietMode {
			fmt.Printf("Downloaded %s (%.2f MB)\n", filename, float64(written)/1024/1024)
		}
		return nil
	},
}

// downloadProgressReader wraps progressReader to change the message
type downloadProgressReader struct {
	*progressReader
}

func (dpr *downloadProgressReader) Read(p []byte) (int, error) {
	n, err := dpr.reader.Read(p)
	dpr.mu.Lock()
	dpr.current += int64(n)
	dpr.mu.Unlock()
	dpr.printProgress()
	return n, err
}

func (dpr *downloadProgressReader) printProgress() {
	dpr.mu.Lock()
	defer dpr.mu.Unlock()

	if dpr.total == 0 {
		return
	}

	percent := int(float64(dpr.current) / float64(dpr.total) * 100)

	// Only print every 5% or at 100%
	if percent != dpr.lastPrinted && (percent%5 == 0 || percent == 100) {
		mb := float64(dpr.current) / 1024 / 1024
		totalMB := float64(dpr.total) / 1024 / 1024
		fmt.Printf("\rDownloading: %d%% (%.2f MB / %.2f MB)", percent, mb, totalMB)
		if percent == 100 {
			fmt.Println() // New line at completion
		}
		dpr.lastPrinted = percent
	}
}

func init() {
	fileObjCreateCmd.Flags().StringVar(&fileName, "name", "", "Name of the file")
	fileObjCreateCmd.Flags().StringVar(&fileObjDescription, "description", "", "A short description for the file")
	fileObjCreateCmd.Flags().StringVar(&filePath, "path", "", "The path on disk to the file")
	fileObjCreateCmd.Flags().StringVar(&fileObjTags, "tags", "", "Comma separated tag list (ie: test,binary,os_type=linux,example)")

	fileObjCreateCmd.MarkFlagRequired("name")
	fileObjCreateCmd.MarkFlagRequired("path")

	fileObjDownloadCmd.Flags().StringVar(&outObjectPath, "out-file", "", "Output file path (optional, defaults to filename from server)")
	fileObjDownloadCmd.Flags().BoolVar(&forceOverwrite, "force-overwrite", false, "Overwrite existing file")
	fileObjDownloadCmd.Flags().BoolVarP(&quietMode, "quiet", "q", false, "Suppress download progress output")

	fileObjUpdateCmd.Flags().StringVar(&fileName, "name", "", "New name of the file")
	fileObjUpdateCmd.Flags().StringVar(&fileObjTags, "tags", "", "Set new tags. The tags are a comma separated list (ie: test,binary,os_type=linux,example)")
	fileObjUpdateCmd.Flags().StringVar(&fileObjDescription, "description", "", "A short description for the file")

	fileObjListCmd.Flags().StringVar(&fileObjTags, "tags", "", "Comma separated list of tags to use as search items (optional)")
	fileObjListCmd.Flags().Int64Var(&fileObjPage, "page", 0, "The file object page to display")
	fileObjListCmd.Flags().Int64Var(&fileObjPageSize, "page-size", 25, "Total number of results per page")

	fileObjectCmd.AddCommand(fileObjCreateCmd)
	fileObjectCmd.AddCommand(fileObjDownloadCmd)
	fileObjectCmd.AddCommand(fileObjUpdateCmd)
	fileObjectCmd.AddCommand(fileObjListCmd)
	fileObjectCmd.AddCommand(fileObjShowCmd)
	fileObjectCmd.AddCommand(fileObjDeleteCmd)

	rootCmd.AddCommand(fileObjectCmd)
}

func formatOneObject(fileObj params.FileObject) {
	if outputFormat == common.OutputFormatJSON {
		printAsJSON(fileObj)
		return
	}
	t := table.NewWriter()
	t.Style().Options.SeparateHeader = true
	header := table.Row{"Field", "Value"}
	t.AppendHeader(header)
	t.AppendRow([]interface{}{"ID", fileObj.ID})
	t.AppendRow([]interface{}{"Name", fileObj.Name})
	t.AppendRow([]interface{}{"Created At", fileObj.CreatedAt})
	t.AppendRow([]interface{}{"Updated At", fileObj.UpdatedAt})
	t.AppendRow([]interface{}{"Size", fileObj.Size})
	t.AppendRow([]interface{}{"SHA256SUM", fileObj.SHA256})
	t.AppendRow([]interface{}{"File Type", fileObj.FileType})
	t.AppendRow([]interface{}{"Description", fileObj.Description})

	if len(fileObj.Tags) > 0 {
		t.AppendRow([]interface{}{"Tags", strings.Join(fileObj.Tags, ", ")})
	}
	t.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, AutoMerge: true},
		{Number: 2, AutoMerge: false, WidthMax: 100},
	})
	fmt.Println(t.Render())
}

func formatFileObjsList(files params.FileObjectPaginatedResponse) {
	if outputFormat == common.OutputFormatJSON {
		printAsJSON(files)
		return
	}
	t := table.NewWriter()
	// Define column count
	numCols := 6
	t.Style().Options.SeparateHeader = true
	t.Style().Options.SeparateRows = true

	// Page header - fill all columns with the same text
	pageHeaderText := fmt.Sprintf("Page %d of %d", files.CurrentPage, files.Pages)
	pageHeader := make(table.Row, numCols)
	for i := range pageHeader {
		pageHeader[i] = pageHeaderText
	}
	t.AppendHeader(pageHeader, table.RowConfig{
		AutoMerge:      true,
		AutoMergeAlign: text.AlignCenter,
	})
	// Column headers
	header := table.Row{"ID", "Name", "Size", "Tags", "Created", "Updated"}
	t.AppendHeader(header)
	// Right-align numeric columns
	t.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, Align: text.AlignRight},
		{Number: 3, Align: text.AlignRight},
	})

	for _, val := range files.Results {
		row := table.Row{val.ID, val.Name, formatSize(val.Size), strings.Join(val.Tags, ", "), val.CreatedAt.Format("2006-01-02 15:04:05"), val.UpdatedAt.Format("2006-01-02 15:04:05")}
		t.AppendRow(row)
	}
	fmt.Println(t.Render())
}

func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
