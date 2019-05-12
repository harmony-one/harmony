package main

import (
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"github.com/iancoleman/strcase"
)

// Reads all .ini files in the `.hmy` folder of root path
// then encodes them as strings literals in `cmd/client/wallet/generated_ini.go`
func main() {
	const root = "../../../"
	hmy := path.Join(root, ".hmy")
	fs, _ := ioutil.ReadDir(hmy)
	out, _ := os.Create("generated_ini.go")
	out.Write([]byte("package main\n\nconst (\n"))
	for _, f := range fs {
		if strings.HasSuffix(f.Name(), ".ini") {
			out.Write([]byte("\t" + strcase.ToLowerCamel("default_"+strings.TrimSuffix(f.Name(), ".ini")+"_ini") + " = `"))
			f, _ := os.Open(path.Join(hmy, f.Name()))
			io.Copy(out, f)
			out.Write([]byte("`\n"))
		}
	}
	out.Write([]byte(")\n"))
}
