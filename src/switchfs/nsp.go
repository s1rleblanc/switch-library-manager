package switchfs

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"

	"github.com/klauspost/compress/zstd"
	"go.uber.org/zap"
)

func ReadNspMetadata(filePath string) (map[string]*ContentMetaAttributes, error) {
	var reader io.ReaderAt
	var closer io.Closer
	var pfs0 *PFS0
	var err error

	if strings.HasSuffix(strings.ToLower(filePath), ".nsz") {
		data, err := decompressNSZ(filePath)
		if err != nil {
			return nil, err
		}
		br := bytes.NewReader(data)
		reader = br
		closer = nil
		pfs0, err = readPfs0(br, 0x0)
	} else {
		pfs0, err = ReadPfs0File(filePath)
		if err != nil {
			return nil, errors.New("Invalid NSP file, reason - [" + err.Error() + "]")
		}
		file, err := OpenFile(filePath)
		if err != nil {
			return nil, err
		}
		reader = file
		closer = file
	}

	if err != nil {
		return nil, err
	}

	if closer != nil {
		defer closer.Close()
	}

	contentMap := map[string]*ContentMetaAttributes{}

	for _, pfs0File := range pfs0.Files {

		fileOffset := int64(pfs0File.StartOffset)

		if strings.Contains(pfs0File.Name, "cnmt.nca") {
			_, section, err := openMetaNcaDataSection(reader, fileOffset)
			if err != nil {
				return nil, err
			}
			currpfs0, err := readPfs0(bytes.NewReader(section), 0x0)
			if err != nil {
				return nil, err
			}
			currCnmt, err := readBinaryCnmt(currpfs0, section)
			if err != nil {
				return nil, err
			}
			if currCnmt.Type != "DLC" {
				nacp, err := ExtractNacp(currCnmt, reader, pfs0, 0)
				if err != nil {
					zap.S().Debug("Failed to extract nacp [%v]\n", err.Error())
				}
				currCnmt.Ncap = nacp
			}

			contentMap[currCnmt.TitleId] = currCnmt

		} /*else if strings.Contains(pfs0File.Name, ".cnmt.xml") {
			xmlBytes := make([]byte, pfs0File.Size)
			_, err = file.ReadAt(xmlBytes, fileOffset)
			if err != nil {
				return nil, err
			}

			currCnmt, err := readXmlCnmt(xmlBytes)
			if err != nil {
				return nil, err
			}
			contentMap[currCnmt.TitleId] = currCnmt
		}*/
	}
	return contentMap, nil

}

func decompressNSZ(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	zr, err := zstd.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer zr.Close()

	data, err := io.ReadAll(zr)
	if err != nil {
		return nil, err
	}
	return data, nil
}
