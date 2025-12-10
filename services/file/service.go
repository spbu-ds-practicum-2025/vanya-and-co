package file

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

type FileService struct {
	BasePath  string
	MaxSizeMB int
}

func New(base string, maxSizeMB int) *FileService {
	os.MkdirAll(base, os.ModePerm)
	return &FileService{BasePath: base, MaxSizeMB: maxSizeMB}
}

// UPLOAD
func (s *FileService) Upload(w http.ResponseWriter, r *http.Request) {

	// ограничение
	r.Body = http.MaxBytesReader(w, r.Body, int64(s.MaxSizeMB<<20))

	r.ParseMultipartForm(10 << 20)

	id := uuid.New().String()

	file, handler, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "no file", 400)
		return
	}
	defer file.Close()

	filename := id + "-" + handler.Filename

	dst := filepath.Join(s.BasePath, filename)

	out, err := os.Create(dst)
	if err != nil {
		http.Error(w, "can't save", 500)
		return
	}
	defer out.Close()

	io.Copy(out, file)

	fmt.Fprintf(w, "uploaded ok: %s\n", filename)
	fmt.Fprintf(w, `<br><a href="/files/list">Show files</a>`)
}

// LIST
func (s *FileService) List(w http.ResponseWriter, r *http.Request) {
	files, _ := os.ReadDir(s.BasePath)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	w.Write([]byte(`
		<html>
		<head>
			<title>Files</title>
			<style>
				body { font-family: Arial; padding: 20px; background:#fafafa; }
				table { width:100%; border-collapse: collapse; }
				td { padding:10px; border-bottom:1px solid #ddd; }
				tr:hover { background:#f0f0f0; }
				a.button{
					padding:6px 12px;
					text-decoration:none;
					border-radius:4px;
					color:white;
					margin-right:6px;
				}
				.btn-view { background:#4CAF50 }
				.btn-download { background:#2196F3 }
				.btn-delete { background:#FF5252 }
			</style>
		</head>
		<body>
			<h1>Files</h1>
			<a href="/upload-form.html">⬆ Upload file</a>
			<br><br>
			<table>
	`))

	for _, f := range files {
		name := f.Name()

		w.Write([]byte(fmt.Sprintf(
			`
				<tr>
					<td>%s</td>
					<td>
						<a class="button btn-view" href="/files/view?name=%s">View</a>
						<a class="button btn-download" href="/files/download?name=%s">Download</a>
						<a class="button btn-delete" href="/files/delete?name=%s">Delete</a>
					</td>
				</tr>
			`,
			name, name, name, name,
		)))
	}

	w.Write([]byte(`
			</table>
		</body>
		</html>
	`))
}


// VIEW — просмотр файлов (картинки откроются прямо в браузере)
func (s *FileService) View(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, filepath.Join(s.BasePath, r.URL.Query().Get("name")))
}

// DOWNLOAD
func (s *FileService) Download(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, filepath.Join(s.BasePath, r.URL.Query().Get("name")))
}

// DELETE
func (s *FileService) Delete(w http.ResponseWriter, r *http.Request) {
	os.Remove(filepath.Join(s.BasePath, r.URL.Query().Get("name")))
	http.Redirect(w, r, "/files/list", 302)
}
