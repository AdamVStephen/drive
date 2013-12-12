// Copyright 2013 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gd

import (
	"fmt"
	"io"
	"os"

	"github.com/rakyll/gd/third_party/github.com/cheggaaa/pb"
	"github.com/rakyll/gd/types"
)

// Pull from remote if remote path exists and in a god context. If path is a
// directory, it recursively pulls from the remote if there are remote changes.
// It doesn't check if there are remote changes if isForce is set.
func (g *Gd) Pull() (err error) {
	if g.context == nil {
		return ErrNoContext
	}

	var r, l *types.File
	if r, err = g.rem.FindByPath(g.opts.Path); err != nil {
		return nil
	}
	absPath := g.context.AbsPathOf(g.opts.Path)
	localinfo, _ := os.Stat(absPath)
	if localinfo != nil {
		l = types.NewLocalFile(absPath, localinfo)
	}

	var cl []*types.Change
	fmt.Println("Resolving...")
	if cl, err = g.resolveChangeListRecv(false, g.opts.Path, r, l); err != nil {
		return
	}
	if ok := printChangeList(cl); !ok {
		return
	}
	return g.playPullChangeList(cl)
}

func (g *Gd) playPullChangeList(cl []*types.Change) (err error) {
	if len(cl) > 0 {
		g.progress = pb.New(len(cl))
		g.progress.Start()
	}
	for _, c := range cl {
		switch c.Op() {
		case types.OpMod:
			g.localMod(c)
		case types.OpAdd:
			g.localAdd(c)
		case types.OpDelete:
			g.localDelete(c)
		}
	}
	if g.progress != nil {
		g.progress.Finish()
	}
	return err
}

func (g *Gd) localMod(change *types.Change) (err error) {
	if g.progress != nil {
		defer g.progress.Increment()
	}
	destAbsPath := g.context.AbsPathOf(change.Path)
	if change.Src.BlobAt != "" {
		// download and replace
		if err = g.download(change); err != nil {
			return
		}
	}
	return os.Chtimes(destAbsPath, change.Src.ModTime, change.Src.ModTime)
}

func (g *Gd) localAdd(change *types.Change) (err error) {
	if g.progress != nil {
		defer g.progress.Increment()
	}
	destAbsPath := g.context.AbsPathOf(change.Path)
	if change.Src.IsDir {
		return os.Mkdir(destAbsPath, os.ModeDir|0755)
	}

	if change.Src.BlobAt != "" {
		// download and create
		if err = g.download(change); err != nil {
			return
		}
	}
	return os.Chtimes(destAbsPath, change.Src.ModTime, change.Src.ModTime)
}

// TODO: no one calls localdelete
func (g *Gd) localDelete(change *types.Change) (err error) {
	if g.progress != nil {
		defer g.progress.Increment()
	}
	return os.RemoveAll(change.Dest.BlobAt)
}

func (g *Gd) download(change *types.Change) (err error) {
	destAbsPath := g.context.AbsPathOf(change.Path)
	var fo *os.File
	fo, err = os.Create(destAbsPath)
	if err != nil {
		return
	}

	// close fo on exit and check for its returned error
	defer func() {
		if err := fo.Close(); err != nil {
			return
		}
	}()

	var blob io.ReadCloser
	defer func() {
		if blob != nil {
			blob.Close()
		}
	}()
	blob, err = g.rem.Download(change.Src.Id)
	if err != nil {
		return err
	}
	_, err = io.Copy(fo, blob)
	return
}
