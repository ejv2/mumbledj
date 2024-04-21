package bot

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Library opening errors.
var (
	ErrExtensionSet      = fmt.Errorf("empty extension set: at least one media extension must be specified")
	ErrExtensionsOverlap = fmt.Errorf("playlist extension set may not overlap media file extension set")
	ErrEmptyLibrary      = fmt.Errorf("library is empty")
)

// A Playable is something which is capable of being added to the queue and
// played using the "localfile" service.
type Playable interface {
	// Files returns a slice of the absolute paths to the media files which
	// are referenced by this playable item.
	//
	// In the case of a single media file, this will be of length one, else
	// one or more.
	Files() []string

	// Title returns the title associated with this playable item.
	//
	// In the case of a single media file, this is extracted from metadata.
	// Else, it is implementation defined.
	Title() string

	// Nested returns any nested playable items. A nested playable item is
	// an item which is nested within this one and which may be played
	// independently from this one, or as part of it as a whole.
	//
	// For single media files, this always returns nil. For directories,
	// this only returns any nested directories.
	Nested() []Library
}

// mediaFile represents a single media file loaded from a library on disk.
type mediaFile struct {
	// Path is the absolute path to this media file.
	path string
	// Title is the title extracted from this file using the file metadata.
	title string
}

// Files always returns a slice of length one containing just this file.
func (m mediaFile) Files() []string {
	return []string{m.path}
}

// Title returns the title of the media file, either determined from metadata
// or from the end of the filepath, whichever is available.
func (m mediaFile) Title() string {
	return m.title
}

// Nested always returns nil; you can't have a nested library inside a file.
func (m mediaFile) Nested() []Library {
	return nil
}

// playlistFile represents a file which contains a list of other media files
// from the same directory as the playlist file itself.
type playlistFile struct {
	paths []string
	title string
}

// Files returns the paths array which was found during parsing. These paths
// are guaranteed to be absolute.
func (p playlistFile) Files() []string {
	return p.paths
}

// Title returns the playlist title. This can be specified using a title
// statement in the file but is normally just the filename of the playlist
// without the extension.
func (p playlistFile) Title() string {
	return p.title
}

// Nested always returns nil; you can't have a nested library inside a file.
func (p playlistFile) Nested() []Library {
	return nil
}

// A Library is a store of playable media items, which can be playlists or
// media files. The store is rooted at a specific directory, which must have
// readable permissions for the bot. Media items are loaded at library
// instantiation and are not re-populated after that point.
type Library struct {
	// entries stores all the playable entries retrieved from the disk.
	entries []Playable
	// nested libraries are directories inside of this directory which can
	// also be accessed for playing.
	nested []Library

	// root is the root directory as specified by the user.
	root string
}

func (l *Library) loadSingleFile(path string) error {
	l.entries = append(l.entries, mediaFile{
		path: path,
		// TODO: Use file metadata
		title: filepath.Base(path),
	})

	return nil
}

// TODO: This function foes not yet do its job!
func (l *Library) loadPlaylistFile(path string) error {
	l.entries = append(l.entries, playlistFile{
		paths: []string{path},
		title: filepath.Base(path),
	})

	return nil
}

// NewLibrary loads and populates a library from the given directory given the
// list of acceptable file extensions. For the acceptable file extensions, a
// list length of zero is interpreted as no acceptable extensions, but an
// extension consisting of a blank string matches any file with no extension at
// all ("my-playlist", for example).
func NewLibrary(root string, extensions []string, playlistExtensions []string) (Library, error) {
	lib := Library{root: root}

	// Reject libraries with no extension set.
	if len(extensions) == 0 {
		return lib, fmt.Errorf("open library %s: %w", root, ErrExtensionSet)
	}

	// Quick lookup tables to check for extensions.
	ext := make(map[string]struct{}, len(extensions))
	for _, e := range extensions {
		ext[e] = struct{}{}
	}
	pext := make(map[string]struct{}, len(playlistExtensions))
	for _, e := range playlistExtensions {
		if _, ok := ext[e]; ok {
			return lib, fmt.Errorf("open library %s: playlist extension %q: %w", root, e, ErrExtensionsOverlap)
		}

		pext[e] = struct{}{}
	}

	fs, err := os.ReadDir(root)
	if err != nil {
		return lib, fmt.Errorf("open library %s: %w", root, err)
	}

	if len(fs) == 0 {
		return lib, fmt.Errorf("open library %s: %s", root, ErrEmptyLibrary)
	}

	for _, f := range fs {
		// If no dot appears we assume that the file is intended to
		// have no extension.
		_, e, _ := strings.Cut(f.Name(), ".")
		path := filepath.Join(root, f.Name())

		if f.IsDir() {
			// If this is a directory, we have a nested library.
			nest, err := NewLibrary(path, extensions, playlistExtensions)
			if err != nil {
				return lib, fmt.Errorf("open library %s: %w", root, err)
			}

			lib.nested = append(lib.nested, nest)
		} else {
			// Else, decide the class of file we have here.
			_, ef := ext[e]
			_, pef := pext[e]
			switch {
			case ef:
				if err := lib.loadSingleFile(path); err != nil {
					return lib, fmt.Errorf("open library %s: file %s: %w", root, f.Name(), err)
				}
			case pef:
				if err := lib.loadPlaylistFile(path); err != nil {
					return lib, fmt.Errorf("open library %s: playlist %s: %w", root, f.Name(), err)
				}
			}
		}
	}

	return lib, nil
}

// Root returns the root directory for the library as specified by the user.
func (l *Library) Root() string {
	return l.root
}

// Files returns the files from this library which may be queued in bulk. This
// may not be used on toplevel libraries, which are not suitable for enqueueing
// for safety.
//
// Libraries contain some special handling to ignore playlist files as they
// don't need to show up file listings when enqueueing.
func (l *Library) Files() []string {
	f := make([]string, 0, len(l.entries))
	for _, e := range l.entries {
		fi, ok := e.(mediaFile)
		if ok {
			f = append(f, fi.path)
		}
	}

	return f
}

// Title returns the title of the library. In the case of a library, the title
// is defined as the basename of the root path.
func (l *Library) Title() string {
	return filepath.Base(l.root)
}

// Nested returns any nested libraries from within this one. These usually take
// the form of directories on disk.
func (l *Library) Nested() []Library {
	return l.nested
}

// String returns a simple textual representation of this library mainly for debugging.
func (l *Library) String() string {
	sb := &strings.Builder{}

	for _, d := range l.nested {
		fmt.Fprintf(sb, "[dir] -----%s-----\n", d.Title())
		fmt.Fprintln(sb, d.String())
	}

	for _, f := range l.entries {
		if _, ok := f.(playlistFile); ok {
			fmt.Fprintf(sb, "[playlist] %s\n", f.Title())
		} else {
			fmt.Fprintln(sb, f.Title())
		}
	}

	return sb.String()
}
