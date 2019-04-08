package vector;

import (
	"time"
	"strings"
	"sort"
	"path/filepath"
	// "fmt"
)

type Vector struct { // раскладка по одному правилу
	files BkpFiles
	pat *Pattern
	intervals []int
	windows []Window  // окна для раскладки
}
type Window struct {
	d1 time.Time // start interval
	d2 time.Time // stop interval
	apply *BkpFile // best match
}
/*
для инициализации окон требуется дата начала
можно инициализировать окна относительно "времени 0", но дата нужна для раскладывания
чтобы не просрать данные самое правильное брать для начала окна дату самого нового файла
если брать текущую - то не обновляющиеся файлы со временем удалятся - это неправильно.

именно поэтому сначала собираются все файлы а потом они раскладываются по окнам
хотя можно было бы построить окна и при добавлении файлов сразу класть в нужное коно
*/

func MakeVector(i []int,  p string, l int) Vector {
	// lng := 0
	// for _, it := range i {
	// 	lng += it
	// }
	lng := len(i)
	return Vector{
		files: make(BkpFiles, 0, l),
		intervals: i,
		pat: makePattern(p),
		windows: make([]Window, lng),
	}
}

func (vtr *Vector) AppendFile(filename string) (string) {
	valid, err := filepath.Match(vtr.pat.pathGlob(), filename)
	if err != nil { return "glob" }
	if !valid { return "miss" }

	time, err := time.Parse(vtr.pat.timeGlob(), vtr.pat.fetchTime(filename))
	if err != nil {
		return "time" 
	} else {
		vtr.files = append(vtr.files, BkpFile{Name: filename, Iat: time})
		return "ok"
	}
}
func (vtr *Vector) MatchFile(filename string) (string) {
	valid, err := filepath.Match(vtr.pat.pathGlob(), filename)
	if err != nil { return "glob" }
	if !valid { return "miss" }

	_, err = time.Parse(vtr.pat.timeGlob(), vtr.pat.fetchTime(filename))
	if err != nil {
		return "time" 
	} else {
		return "ok"
	}
}
func (vtr *Vector) SortFiles() {
	sort.Sort(vtr.files)
}
func (vtr *Vector) FillWindows() {
	// начало - максимальное значение имеющейся даты
	// TODO ввести таймаут для контроля устаревания окна (или считать оттекущего дня)
	/*
	last file d-15, date d-16
	d2 d-16 -1sec
	d1 d-15
	*/
	diffunit := 24* time.Hour
	date := vtr.files[len(vtr.files)-1].Iat.Add(diffunit)

	wix := 0 // сквозной номер окна - для нумерации в windows
	for _, it := range vtr.intervals {
		diff := diffunit * time.Duration(it)
		// создаем записи
		{
			vtr.windows[wix].d2 = date.Add(-time.Duration(1)) // конец
			date = date.Add(-diff)
			vtr.windows[wix].d1 = date // начало
			wix++
		}
	}
}
func (vtr *Vector) ProcessFiles() {
	/* пользуемся фактом сортировки (для этого и сортировали):
	нужен самый старый файл - по сортировке он первый.
	если файл подошел - его нельзя перезаписывать, 
	так мы сохраним самый старый файл в интервале 
	(и он корректно перейдет в следующий интервал). 
	соответсвенно в это окно больше не заглядываем*/
	// конец отсматриваемого списка окон, когда в окне нацден подходящий элемент граница сдвигается
	jstart := len(vtr.windows)-1
	for ix, it := range vtr.files {
		for jx := jstart; jx >= 0; jx-- { // окна от старшего. разбираем от самого старшего файла
			jt := vtr.windows[jx]
			if it.Iat == jt.d1 || 
				(it.Iat.After(jt.d1) && it.Iat.Before(jt.d2)) {
				vtr.windows[jx].apply = &vtr.files[ix]
				vtr.files[ix].keep = true
				jstart-- // закрываем окно
				continue // экономим ресурсы - окна не перекрываются
			}
		}
	}
}

func (vtr *Vector) GetUsedFiles() []string {
	ret := make(ArrString, 0)
	for _, it := range vtr.files {
		if it.keep {
			ret = append(ret, it.Name)
		}
	}
	sort.Sort(ret)
	return ret
}

func (vtr *Vector) GetUnusedFiles() []string {
	ret := make(ArrString, 0)
	for _, it := range vtr.files {
		if !it.keep {
			ret = append(ret, it.Name)
		}
	}
	sort.Sort(ret)
	return ret
}
func (vtr *Vector) Desc() string {
	return vtr.pat.Raw
}
// --------------------------------------------------------
type BkpFiles []BkpFile

type BkpFile struct {
	Name string
	Iat time.Time
	keep bool
}

func (s BkpFiles) Len() int {
    return len(s)
}
func (s BkpFiles) Swap(i, j int) {
    s[i], s[j] = s[j], s[i]
}
func (s BkpFiles) Less(i, j int) bool {
    return s[i].Iat.Before(s[j].Iat)
}

type ArrString []string
func (s ArrString) Len() int {
    return len(s)
}
func (s ArrString) Swap(i, j int) {
    s[i], s[j] = s[j], s[i]
}
func (s ArrString) Less(i, j int) bool {
    return s[i] > s[j]
}
// --------------------------------------------------------
type Pattern struct {
	Raw string
	d1 int
	d2 int
}
func makePattern(in string) *Pattern {
	date1 := strings.Index(in, "{")
	date2 := strings.Index(in, "}")
	return &Pattern{in, date1, date2}
}

func (p *Pattern) pathGlob() string {
	return p.Raw[:p.d1] + "*" + p.Raw[p.d2+1:]
}
func (p *Pattern) timeGlob() string {
	return p.Raw[p.d1+1:p.d2]
}
func (p *Pattern) fetchTime(in string) string {
	if len(in) < p.d1 || len(in) < p.d2-1 {
		return ""
	}
	return in[p.d1:p.d2-1]
}
// --------------------------------------------------------
