package install

import (
	"bytes"
	"io"
	"net/http"
	"runtime"
	"time"

	"github.com/inherelab/eget/internal/install/detect"
	forge "github.com/inherelab/eget/internal/source/forge"
	sourcegithub "github.com/inherelab/eget/internal/source/github"
	sourcesf "github.com/inherelab/eget/internal/source/sourceforge"
	"github.com/inherelab/eget/internal/source/urltemplate"
)

func NewDefaultService(githubGetter sourcegithub.HTTPGetter, binaryModTime func(tool, output string) time.Time) *Service {
	return &Service{
		BinaryModTime: binaryModTime,
		GitHubGetter:  githubGetter,
		GitHubGetterFactory: func(opts Options) sourcegithub.HTTPGetter {
			return NewHTTPGetter(opts)
		},
		ForgeGetterFactory: func(opts Options) forge.HTTPGetter {
			return NewHTTPGetter(opts)
		},
		SourceForgeGetterFactory: func(opts Options) sourcesf.HTTPGetter {
			return NewHTTPGetter(opts)
		},
		AllDetectorFactory: detect.NewAllDetector,
		SystemDetectorFactory: func(goos, goarch string) (Detector, error) {
			libc := ""
			if goos == "linux" && runtime.GOOS == "linux" {
				libc = urltemplate.DetectLibc()
			}
			return detect.NewSystemDetectorWithLibc(goos, goarch, libc)
		},
		AssetDetectorFactory: detect.NewAssetDetector,
		DetectorChainFactory: detect.NewChain,
		Sha256VerifierFactory: func(expected string) (Verifier, error) {
			return newSha256Verifier(expected)
		},
		Sha256AssetVerifierFactory: func(assetURL string, opts Options) Verifier {
			getter := githubGetter
			if opts.CacheMirror.Active() {
				getter = HTTPGetterFunc(func(url string) (*http.Response, error) {
					downloaded, err := (&InstallRunner{}).downloadBody(url, opts)
					if err != nil {
						return nil, err
					}
					return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(downloaded.Body))}, nil
				})
			} else if getter == nil {
				getter = NewHTTPGetter(opts)
			}
			return &sha256AssetVerifier{AssetURL: assetURL, Getter: getter}
		},
		Sha256PrinterFactory: func() Verifier {
			return &sha256Printer{}
		},
		NoVerifierFactory: func() Verifier {
			return &noVerifier{}
		},
		DownloadOnlyExtractorFactory: func(name string) any {
			return NewDownloadOnlyExtractor(name)
		},
		GlobChooserFactory: func(pattern string) (any, error) {
			return NewFileChooser(pattern)
		},
		BinaryChooserFactory: func(tool string) any {
			return NewBinaryChooser(tool)
		},
		ExtractorFactory: func(filename, tool string, chooser any) any {
			return NewExtractor(filename, tool, chooser.(Chooser))
		},
		System7zPathResolver: resolveSystem7zPath,
		System7zExtractorFactory: func(filename, tool string, chooser Chooser, exe string) Extractor {
			return NewSystem7zExtractor(filename, tool, chooser, exe)
		},
	}
}
