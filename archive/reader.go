package archive

import (
	"archive/zip"
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/juruen/rmapi/encoding/rm"
	"github.com/juruen/rmapi/util"
)

// Read fills a Zip parsing a Remarkable archive file.
func (z *Zip) Read(r io.ReaderAt, size int64) error {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return err
	}

	// reading content first because it contains the number of pages
	if err := z.readContent(zr); err != nil {
		return err
	}

	if err := z.readPayload(zr); err != nil {
		return err
	}

	if err := z.readMetadata(zr); err != nil {
		return err
	}

	//uploading and then downloading a file results in 0 pages
	if len(z.Pages) == 0 {
		return nil
	}

	if err := z.readPagedata(zr); err != nil {
		return err
	}

	if err := z.readData(zr); err != nil {
		return err
	}

	if err := z.readHighlights(zr); err != nil {
		return err
	}

	if err := z.readThumbnails(zr); err != nil {
		return err
	}

	return nil
}

// readContent reads the .content file contained in an archive and the UUID
func (z *Zip) readContent(zr *zip.Reader) error {
	files, err := zipExtFinder(zr, ".content")
	if err != nil {
		return err
	}

	if len(files) != 1 {
		return errors.New("archive does not contain a unique content file")
	}

	contentFile := files[0]
	file, err := contentFile.Open()
	if err != nil {
		return err
	}
	defer file.Close()

	bytes, err := ioutil.ReadAll(file)
	if err != nil {
		return err
	}

	if err = json.Unmarshal(bytes, &z.Content); err != nil {
		return err
	}
	p := contentFile.FileInfo().Name()
	id, _ := util.DocPathToName(p)
	z.UUID = id

	// instantiate the slice of pages
	pagesLength := len(z.Content.Pages)
	if pagesLength > 0 {
		z.Pages = make([]Page, pagesLength)
		z.pageMap = make(map[string]int)
		for index, pageUUID := range z.Content.Pages {
			z.pageMap[pageUUID] = index
		}
	}

	return nil
}

// readPagedata reads the .pagedata file contained in an archive
// and iterate to gather which template was used for each page.
func (z *Zip) readPagedata(zr *zip.Reader) error {
	files, err := zipExtFinder(zr, ".pagedata")
	if err != nil {
		return err
	}

	if len(files) != 1 {
		return errors.New("archive does not contain a unique pagedata file")
	}

	file, err := files[0].Open()
	if err != nil {
		return err
	}
	defer file.Close()

	// iterate pagedata file lines
	sc := bufio.NewScanner(file)
	var i int = 0
	for sc.Scan() {
		line := sc.Text()
		z.Pages[i].Pagedata = line
		i++
	}

	if err := sc.Err(); err != nil {
		return err
	}

	return nil
}

// readPayload tries to extract the payload from an archive if it exists.
func (z *Zip) readPayload(zr *zip.Reader) error {
	ext := z.Content.FileType
	files, err := zipExtFinder(zr, "."+ext)
	if err != nil {
		return err
	}

	// return if not found
	if len(files) != 1 {
		return nil
	}

	file, err := files[0].Open()
	if err != nil {
		return err
	}
	defer file.Close()

	z.Payload, err = ioutil.ReadAll(file)
	if err != nil {
		return err
	}

	return nil
}

// readData extracts existing .rm files from an archive.
func (z *Zip) readData(zr *zip.Reader) error {
	files, err := zipExtFinder(zr, ".rm")
	if err != nil {
		return err
	}

	for _, file := range files {
		name, _ := splitExt(file.FileInfo().Name())
		idx, ok := z.pageMap[name]

		if !ok {
			idx, err = strconv.Atoi(name)
			if err != nil {
				return errors.New("error in metadata .rm filename, not a numnber: " + name)
			}
		}

		if len(z.Pages) <= idx {
			return errors.New("page not found")
		}

		r, err := file.Open()
		if err != nil {
			return err
		}

		bytes, err := ioutil.ReadAll(r)
		if err != nil {
			return err
		}

		z.Pages[idx].Data = rm.New()
		z.Pages[idx].Data.UnmarshalBinary(bytes)
		if err != nil {
			return err
		}
	}

	return nil
}

func (z *Zip) readHighlights(zr *zip.Reader) error {
	files, err := zipDirExtFinder(zr, z.UUID+".highlights", ".json")
	if err != nil {
		return err
	}

	for _, file := range files {
		name, _ := splitExt(file.FileInfo().Name())
		r, err := file.Open()
		if err != nil {
			return err
		}
		index, ok := z.pageMap[name]
		if !ok {
			continue
		}

		bytes, err := ioutil.ReadAll(r)
		if err != nil {
			return err
		}

		var highlights HighlihgtsContainer
		err = json.Unmarshal(bytes, &highlights)
		if err != nil {
			return err
		}
		z.Pages[index].Highlights = &highlights
	}
	return nil
}

// readThumbnails extracts existing thumbnails from an archive.
func (z *Zip) readThumbnails(zr *zip.Reader) error {
	files, err := zipExtFinder(zr, ".jpg")
	if err != nil {
		return err
	}

	for _, file := range files {
		name, _ := splitExt(file.FileInfo().Name())

		idx, err := strconv.Atoi(name)
		if err != nil {
			return errors.New("error in .jpg filename")
		}

		if len(z.Pages) <= idx {
			return errors.New("page not found")
		}

		r, err := file.Open()
		if err != nil {
			return err
		}

		z.Pages[idx].Thumbnail, err = ioutil.ReadAll(r)
		if err != nil {
			return err
		}
	}

	return nil
}

// readMetadata extracts existing .json metadata files from an archive.
func (z *Zip) readMetadata(zr *zip.Reader) error {
	files, err := zipExtFinder(zr, ".json")
	if err != nil {
		return err
	}

	for _, file := range files {
		name := strings.TrimSuffix(file.FileInfo().Name(), "-metadata.json")

		idx, ok := z.pageMap[name]
		if !ok {
			idx, err = strconv.Atoi(name)
			if err != nil {
				return errors.New("error in metadata .json filename, not a numnber: " + name)
			}
		}

		if len(z.Pages) <= idx {
			return errors.New("page not found")
		}

		r, err := file.Open()
		if err != nil {
			return err
		}

		bytes, err := ioutil.ReadAll(r)
		if err != nil {
			return err
		}

		err = json.Unmarshal(bytes, &z.Pages[idx].Metadata)
		if err != nil {
			return err
		}
	}

	return nil
}

// splitExt splits the extension from a filename
func splitExt(name string) (string, string) {
	ext := filepath.Ext(name)
	return name[0 : len(name)-len(ext)], ext
}

// zipExtFinder searches for a file matching the substr pattern
// in a zip file.
func zipExtFinder(zr *zip.Reader, ext string) ([]*zip.File, error) {
	var files []*zip.File

	for _, file := range zr.File {
		parentFolderName := path.Dir(file.FileHeader.Name)
		if strings.HasSuffix(parentFolderName, ".highlights") {
			continue
		}
		filename := file.FileInfo().Name()
		if _, e := splitExt(filename); e == ext {
			files = append(files, file)
		}
	}

	return files, nil
}

func zipDirExtFinder(zr *zip.Reader, folder, ext string) ([]*zip.File, error) {
	var files []*zip.File

	for _, file := range zr.File {
		parentFolderName := path.Dir(file.FileHeader.Name)
		if !strings.HasPrefix(parentFolderName, folder) {
			continue
		}

		filename := file.FileInfo().Name()
		if _, e := splitExt(filename); e == ext {
			files = append(files, file)
		}
	}

	return files, nil
}
