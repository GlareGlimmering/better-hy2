package frontend

import (
	"bytes"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"path"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/gin-gonic/gin"
)

var embeddedAssetReferencePattern = regexp.MustCompile(`["']((?:\.\.?/|/__dynamic_base__/|/assets/)[^"'?#]+\.(?:js|css|svg|png|jpe?g|gif|webp|ico|woff2?|ttf|eot)(?:\?[^"']*)?)["']`)

func TestProductionFrontendIsEmbedded(t *testing.T) {
	index, err := staticFiles.ReadFile("dist/index.html")
	if err != nil {
		t.Fatalf("read embedded production index: %v", err)
	}
	if len(index) == 0 {
		t.Fatal("embedded production index is empty")
	}
	if !bytes.Contains(index, []byte(`<div id="app"></div>`)) {
		t.Fatal("embedded production index does not contain the application root")
	}
}

func TestEmbeddedProductionAssetReferences(t *testing.T) {
	if err := validateEmbeddedAssetReferences(staticFiles, "dist"); err != nil {
		t.Fatal(err)
	}
}

func TestEmbeddedUnderscoreChunksAreServed(t *testing.T) {
	chunks, err := fs.Glob(staticFiles, "dist/assets/_*.js")
	if err != nil {
		t.Fatalf("find embedded underscore chunks: %v", err)
	}
	if len(chunks) == 0 {
		t.Fatal("production build contains no embedded underscore chunks")
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	InitFrontend(router, "/")
	for _, chunk := range chunks {
		request := httptest.NewRequest(http.MethodGet, "/assets/"+path.Base(chunk), nil)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Errorf("GET /assets/%s returned %d", path.Base(chunk), response.Code)
		}
	}
}

func TestAssetReferenceValidationDetectsMissingUnderscoreChunk(t *testing.T) {
	assets := fstest.MapFS{
		"dist/index.html":             {Data: []byte(`<script type="module" src="/__dynamic_base__/assets/dashboard.js"></script>`)},
		"dist/assets/dashboard.js":    {Data: []byte(`import "./_initCloneObject-fa572840.js"`)},
		"dist/assets/application.css": {Data: []byte(`body { background-image: url("./background.svg"); }`)},
		"dist/assets/background.svg":  {Data: []byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`)},
	}

	err := validateEmbeddedAssetReferences(assets, "dist")
	if err == nil || !strings.Contains(err.Error(), "_initCloneObject-fa572840.js") {
		t.Fatalf("expected missing underscore chunk error, got %v", err)
	}

	assets["dist/assets/_initCloneObject-fa572840.js"] = &fstest.MapFile{Data: []byte(`export const clone = value => value`)}
	if err := validateEmbeddedAssetReferences(assets, "dist"); err != nil {
		t.Fatalf("expected complete asset graph to pass: %v", err)
	}
}

func validateEmbeddedAssetReferences(assets fs.FS, root string) error {
	files := make(map[string]struct{})
	var sourceFiles []string
	if err := fs.WalkDir(assets, root, func(filePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		files[filePath] = struct{}{}
		switch path.Ext(filePath) {
		case ".html", ".js", ".css":
			sourceFiles = append(sourceFiles, filePath)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("walk embedded production assets: %w", err)
	}

	for _, sourcePath := range sourceFiles {
		content, err := fs.ReadFile(assets, sourcePath)
		if err != nil {
			return fmt.Errorf("read embedded asset %s: %w", sourcePath, err)
		}
		for _, match := range embeddedAssetReferencePattern.FindAllSubmatch(content, -1) {
			reference := strings.SplitN(string(match[1]), "?", 2)[0]
			target := resolveEmbeddedAssetReference(root, sourcePath, reference)
			if _, ok := files[target]; !ok {
				return fmt.Errorf("embedded asset %s references missing file %s", sourcePath, target)
			}
		}
	}
	return nil
}

func resolveEmbeddedAssetReference(root, sourcePath, reference string) string {
	switch {
	case strings.HasPrefix(reference, "/__dynamic_base__/"):
		return path.Join(root, strings.TrimPrefix(reference, "/__dynamic_base__/"))
	case strings.HasPrefix(reference, "/assets/"):
		return path.Join(root, strings.TrimPrefix(reference, "/"))
	default:
		return path.Clean(path.Join(path.Dir(sourcePath), reference))
	}
}
