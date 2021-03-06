package main

import (
	"flag"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"

	"gopkg.in/yaml.v2"
)

type config struct {
	InputFolder  string
	OutputFolder string
	ImageFolder  string
	ManifestFile string
	SizesAttr    string
	OptimCommand []string
	// A number between 0-1 where if the downscaling is greater
	// than this fraction of the width it doesn't bother.
	ScaleThreshold    float64
	JpgScaleThreshold float64
	JpgQuality        int
	DoCopy            bool
	Widths            []int
	Blacklist         []string
	basePath          string
	doCulling         bool
}

func (conf *config) ImageFolderPath() string {
	return path.Join(conf.basePath, conf.OutputFolder, conf.ImageFolder)
}

func (conf *config) InputFolderPath() string {
	return path.Join(conf.basePath, conf.InputFolder)
}

func copyFile(source string, dest string) error {
	sf, err := os.Open(source)
	if err != nil {
		return err
	}
	defer sf.Close()
	df, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer df.Close()
	_, err = io.Copy(df, sf)
	return err
}

func readFileBytes(path string) (bytes []byte, err error) {
	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return
	}
	defer f.Close()

	bytes, err = ioutil.ReadAll(f)
	return
}

func readConfig(basePath string) (conf config, err error) {
	conf = config{
		InputFolder:  "_site",
		OutputFolder: "_fastsite",
		ImageFolder:  "assets/images",
		ManifestFile: "enfasten_manifest.yml",
		SizesAttr:    "",
		OptimCommand: nil,
		DoCopy:       true,
		// ManifestFile:   "",
		ScaleThreshold:    0.9,
		JpgScaleThreshold: 0.7,
		JpgQuality:        90,
		Widths:            []int{},
	}

	bytes, err := readFileBytes(path.Join(basePath, "enfasten.yml"))
	if err != nil {
		return
	}
	err = yaml.Unmarshal(bytes, &conf)
	return
}

func buildFastSite(basePath string, doCulling bool) (err error) {
	conf, err := readConfig(basePath)
	if err != nil {
		return
	}

	conf.basePath = basePath
	conf.doCulling = doCulling

	foundImages, err := discoverImages(&conf, path.Join(conf.basePath, conf.InputFolder))
	if err != nil {
		return
	}

	manifestPath := conf.ManifestFile
	if manifestPath != "" {
		manifestPath = path.Join(conf.basePath, manifestPath)
	}

	oldManifest, err := readManifest(manifestPath)
	if err != nil {
		return
	}

	err = os.MkdirAll(conf.ImageFolderPath(), os.ModePerm)
	if err != nil {
		return
	}

	newManifest, pathToSlug, err := buildNewManifest(&conf, foundImages, oldManifest)
	if err != nil {
		return
	}

	err = saveManifest(manifestPath, newManifest)
	if err != nil {
		return
	}

	// TODO also clean up files when not copying
	if !conf.DoCopy {
		return
	}

	// fmt.Printf("%v\n", pathToSlug)

	transformConf := transformConfig{
		config:     &conf,
		manifest:   newManifest,
		pathToSlug: pathToSlug,
	}
	whitelist, err := transferAndTransformAll(&transformConf)
	if err != nil {
		return
	}

	// whitelist all our files
	imageFolder := conf.ImageFolderPath()
	for _, bImg := range newManifest {
		for _, bImgFile := range bImg.Files {
			whitelist = append(whitelist, path.Join(imageFolder, bImgFile.FileName))
		}
	}

	// fmt.Printf("Keep: %v\n", whitelist)

	err = deleteNonWhitelist(&conf, whitelist)

	return
}

func main() {
	basePath := flag.String("basepath", ".", "The folder in which to search for enfasten.yml")
	cull := flag.Bool("cull", false, "Whether to cull inefficient images this run")
	flag.Parse()
	err := buildFastSite(*basePath, *cull)
	if err != nil {
		log.Fatal("FATAL ERROR: ", err)
	}
}
