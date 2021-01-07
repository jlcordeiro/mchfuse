/*
Copyright 2020 Marco Nenciarini <mnencia@gmail.com>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package mch

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"strconv"

	"github.com/hanwen/go-fuse/v2/fuse"
)

const DirectoryMimeType = "application/x.wd.dir"
const FileFields = "id,eTag,parentID,childCount,mimeType,name,size,mTime,cTime"

var ErrorInvalidOperation = errors.New("invalid operation")

type File struct {
	ID         string  `json:"id"`
	ETag       string  `json:"eTag"`
	ParentID   string  `json:"parentID"`
	ChildCount int     `json:"childCount,omitempty"`
	MimeType   string  `json:"mimeType"`
	Name       string  `json:"name"`
	Size       uint64  `json:"size,omitempty"`
	MTime      ISOTime `json:"mTime"`
	CTime      ISOTime `json:"cTime"`
	client     *Client
	device     *Device
	cache      *CacheFile
}

type FileList struct {
	Files     []File `json:"files"`
	PageToken string `json:"pageToken"`
	ETag      string `json:"eTag"`
	client    *Client
	device    *Device
}

func (f *File) IsDirectory() bool {
	return f.MimeType == DirectoryMimeType
}

func (f *File) ListDirectory() (map[string]File, error) {
	if !f.IsDirectory() {
		return nil, fmt.Errorf("%s is not a directory: %w", f.Name, ErrorInvalidOperation)
	}

	files := make(map[string]File)
	pageToken := ""

	for {
		fileList, err := f.device.fileSearchParents(f.ID, pageToken)
		if err != nil {
			return nil, err
		}

		for _, item := range fileList.Files {
			files[item.Name] = item
		}

		pageToken = fileList.PageToken
		if pageToken == "" {
			break
		}
	}

	return files, nil
}

func (f *File) LookupDirectory(name string) (*File, error) {
	if !f.IsDirectory() {
		return nil, fmt.Errorf("%s is not a directory: %w", f.Name, ErrorInvalidOperation)
	}

	child, err := f.device.fileSearchParentAndName(f.ID, name)
	if err != nil {
		return nil, err
	}

	return child, nil
}

func (f *File) FileExists() bool {
	_, err := f.device.fileByID(f.ID, f)
	if err == nil {
		return true
	}

	_, err = f.device.resumableFileByID(f.ID, f)
	if err == nil {
		return true
	}

	return false
}

func (f *File) Delete() error {
	resp, err := f.device.api(
		"DELETE",
		fmt.Sprintf("/v2/files/%s", f.ID),
		nil,
		nil,
	)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusAccepted:
		// Deleted asynchronously
	case http.StatusNoContent:
		// Deleted synchronously
	case http.StatusNotFound:
		// File was not there
	default:
		return fmt.Errorf(
			"status code %v deleting file %v at %v: %w",
			resp.StatusCode,
			f.ID,
			resp.Request.URL,
			ErrorUnexpectedStatusCode,
		)
	}

	return nil
}

func (f *File) Rename(newParent *File, newName string) error {
	reqJSON := map[string]interface{}{
		"parentID": newParent.ID,
		"name":     newName,
	}

	resp, err := f.patch(reqJSON)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf(
			"status code %v moving %v under %v named %v at %v: %w",
			resp.StatusCode,
			f.ID,
			newParent.ID,
			newName,
			resp.Request.URL,
			ErrorUnexpectedStatusCode,
		)
	}

	return nil
}

func (f *File) patch(reqJSON map[string]interface{}) (*http.Response, error) {
	data, err := json.Marshal(reqJSON)
	if err != nil {
		return nil, err
	}

	resp, err := f.device.api(
		"PATCH",
		fmt.Sprintf("/v2/files/%s", f.ID),
		bytes.NewBuffer(data),
		func(req *http.Request) {
			req.Header.Add("Content-Type", "application/json")
		},
	)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (f *File) CreateDirectory(name string) (*File, error) {
	reqJSON := map[string]string{
		"parentID": f.ID,
		"name":     name,
		"mimeType": "application/x.wd.dir",
	}

	multipartBody, err := NewMultipartBody(reqJSON)
	if err != nil {
		return nil, err
	}

	resp, err := f.device.api(
		"POST",
		"/v2/files",
		multipartBody.Buffer(),
		func(req *http.Request) {
			multipartBody.AddContentType(req)
		},
	)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf(
			"status code %v creating directory %v under %v at %v: %w",
			resp.StatusCode,
			name,
			f.ID,
			resp.Request.URL,
			ErrorUnexpectedStatusCode,
		)
	}

	location := resp.Header.Get("Location")
	newID := path.Base(location)

	return f.device.fileByID(newID, &File{})
}

func (f *File) Read(dest []byte, offset int64) (int, error) {
	if f.IsDirectory() {
		return 0, fmt.Errorf("%s is a directory: %w", f.Name, ErrorInvalidOperation)
	}

	size := int64(len(dest))

	if size == 0 {
		return 0, nil
	}

	resp, err := f.device.api(
		"GET",
		fmt.Sprintf("/v3/files/%s/content", f.ID),
		nil,
		func(req *http.Request) {
			endRange := offset + size - 1
			req.Header.Add("Range", fmt.Sprintf("bytes=%d-%d", offset, endRange))
		},
	)
	if err != nil {
		return 0, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent {
		return 0, fmt.Errorf(
			"status code %v reading file %v offset %v size %v at %v: %w",
			resp.StatusCode,
			f.ID,
			offset,
			size,
			resp.Request.URL,
			ErrorUnexpectedStatusCode,
		)
	}

	b := bytes.NewBuffer(dest[:0])

	n, err := io.Copy(b, resp.Body)
	if err != nil {
		return 0, err
	}

	return int(n), nil
}

func (f *File) Create(name string) (*File, error) {
	reqJSON := map[string]string{
		"parentID": f.ID,
		"name":     name,
	}

	multipartBody, err := NewMultipartBody(reqJSON)
	if err != nil {
		return nil, err
	}

	resp, err := f.device.api(
		"POST",
		"/v2/files/resumable",
		multipartBody.Buffer(),
		func(req *http.Request) {
			multipartBody.AddContentType(req)

			q := req.URL.Query()
			req.URL.RawQuery = q.Encode()
		},
	)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf(
			"status code %v creating file %v under %v at %v: %w",
			resp.StatusCode,
			name,
			f.ID,
			resp.Request.URL,
			ErrorUnexpectedStatusCode,
		)
	}

	location := resp.Header.Get("Location")
	newID := path.Base(location)

	return f.device.resumableFileByID(newID, &File{})
}

func (f *File) upload(
	method string,
	path_format string,
	expected_http_code int,
	data []byte,
	offset int64,
	file_complete bool,
) error {
	resp, err := f.device.api(
		method,
		fmt.Sprintf(path_format, f.ID),
		bytes.NewBuffer(data),
		func(req *http.Request) {
			q := req.URL.Query()
			q.Add("done", strconv.FormatBool(file_complete))
			q.Add("offset", strconv.FormatInt(offset, 10))
			req.URL.RawQuery = q.Encode()
		},
	)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != expected_http_code {
		return fmt.Errorf(
			"status code %v writing file %v at %v: %w",
			resp.StatusCode,
			f.ID,
			resp.Request.URL,
			ErrorUnexpectedStatusCode,
		)
	}

	return nil
}

func (f *File) postResumable(data []byte, offset int64, file_complete bool) error {
	return f.upload("PUT", "/v2/files/%s/resumable/content",
		http.StatusNoContent, data, offset, file_complete)
}

func (f *File) Append(data []byte) error {
	return f.upload("POST", "/v2/files/%s/resumable",
		http.StatusCreated, data, int64(f.Size), true)
}

func (f *File) FlushCache(file_complete bool) error {
	if f.cache == nil || f.cache.Length() == 0 {
		// nothing cached left to write
		return nil
	}

	defer f.cache.Reset()
	return f.postResumable(f.cache.Content(), f.cache.Offset(), file_complete)
}

// concurrent write operations to the same File arrive sequentially.
// a mutex will lock the second write from starting until the first ends.
// e.g. 2 writes to the same file, 1 write + 1 delete, etc
func (f *File) Write(data []byte, offset int64) error {
	skip_cache := f.cache == nil && len(data) < fuse.MAX_KERNEL_WRITE
	if skip_cache {
		return f.postResumable(data, offset, true)
	}

	// if we get here the file was big enough not to fit into one fuse op
	if f.cache == nil {
		f.cache = new(CacheFile)
	}

	f.cache.Add(data, offset)

	if f.cache.Full() == true {
		return f.FlushCache(false)
	}

	return nil
}

func (f *File) Truncate(offset int64) error {
	resp, err := f.device.api(
		"POST",
		fmt.Sprintf("/v2/files/%s/resumable", f.ID),
		nil,
		func(req *http.Request) {
			q := req.URL.Query()
			q.Add("done", "true")
			q.Add("truncate", "true")
			q.Add("offset", strconv.FormatInt(offset, 10))
			req.URL.RawQuery = q.Encode()
		},
	)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf(
			"status code %v writing file %v at %v: %w",
			resp.StatusCode,
			f.ID,
			resp.Request.URL,
			ErrorUnexpectedStatusCode,
		)
	}

	return nil
}

func (f *File) SetMeta(reqJSON map[string]interface{}) error {
	resp, err := f.patch(reqJSON)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf(
			"status code %v patching %v at %v: %w",
			resp.StatusCode,
			f.ID,
			resp.Request.URL,
			ErrorUnexpectedStatusCode,
		)
	}

	return nil
}
