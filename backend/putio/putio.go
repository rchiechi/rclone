// Package putio provides an native interface to put.io
package putio

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"fmt"
	gohash "hash"
	"io"
	"net/http"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

//    "github.com/ncw/rclone/backend/b2/api"
    "golang.org/x/oauth2"
    putioapi "github.com/putdotio/go-putio/putio"
	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/accounting"
	"github.com/ncw/rclone/fs/config/configmap"
	"github.com/ncw/rclone/fs/config/configstruct"
	"github.com/ncw/rclone/fs/fserrors"
	"github.com/ncw/rclone/fs/fshttp"
	"github.com/ncw/rclone/fs/hash"
	"github.com/ncw/rclone/fs/walk"
	"github.com/ncw/rclone/lib/pacer"
	"github.com/ncw/rclone/lib/rest"
	"github.com/pkg/errors"
)
// https://github.com/cenkalti/putio.py/blob/master/putiopy.py
const (
    api_base            = "https://api.put.io/v2"
    upload_base         = "https://upload.put.io"
    base_url            = api_base
    upload_url          = upload_base + "/v2/files/upload"
    tus_upload_url      = upload_base + "/files/"
    access_token_url    = api_base + "/oauth2/access_token"
    authentication_url  = api_base + "/oauth2/authenticate"
    authorization_url   = api_base + "/oauth2/authorizations/clients/%s/%s"
	defaultEndpoint     = "https://api.put.io/v2"
	rootDir             = 0
)


// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "putio",
		Description: "put.io",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name:     "access_token",
            Help:     "put.io oath token (get one here:https://app.put.io/settings/account/oauth/apps)",
			Required: true
		}}
}



// Fs represents a remote putio endpoint
type Fs struct {
	name          string                       // name of this remote
	srv           *putio.Client                // the connection to the putio API
    root          string                       // root directory
}

// Object describes a putio object
//type Object struct {
//	fs       *Fs       // what this object is part of
//	mimeType string    // Content-Type of the object
//	parentid   string    // The parent remote path
//	id       string    // id of the object
//	modTime  time.Time // The modified time of the object if known
//	crc32     string    // crc32 hash
//	size     int64     // Size of the object
//    file_type string   // File type
//    extension string   // File extension
//}


//https://github.com/putdotio/go-putio/blob/master/putio/types.go
type Object struct {
	fs       *Fs       // what this object is part of
	ID                int64  `json:"id"`
	Name              string `json:"name"`
	Size              int64  `json:"size"`
	ContentType       string `json:"content_type"`
	CreatedAt         *Time  `json:"created_at"`
	FirstAccessedAt   *Time  `json:"first_accessed_at"`
	ParentID          int64  `json:"parent_id"`
	Screenshot        string `json:"screenshot"`
	OpensubtitlesHash string `json:"opensubtitles_hash"`
	IsMP4Available    bool   `json:"is_mp4_available"`
	Icon              string `json:"icon"`
	CRC32             string `json:"crc32"`
	IsShared          bool   `json:"is_shared"`
}



//	NewFs func(name string, root string, config configmap.Mapper) (Fs, error) `json:"-"`

// NewFs constructs an Fs from the path, container:path
func NewFs(name string, root string, config configmap.Mapper) (fs.Fs, error) {
    tokenSource := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: config.Get('access_token')})
    oauthClient := oauth2.NewClient(oauth2.NoContext, tokenSource)

    client := putio.NewClient(oauthClient)

    root, err := client.Files.Get(context.Background(), rootDir)
    if err != nil {
        return err
    }

	root = client.Get(rootID).name

	f := &Fs{
		name:   name,
		root:   root,
		srv:    client
	}
	f.features = (&fs.Features{
		CaseInsensitive:         false,
		CanHaveEmptyDirectories: true,
		ReadMimeType:            true,
		WriteMimeType:           false,
	}).Fill(f)
	return f, nil
}

// https://github.com/igungor/go-putio/blob/master/putio/files.go
// IsDir reports whether the file is a directory.
func (f *Object) IsDir() bool {
	return f.ContentType == "application/x-directory"
}


