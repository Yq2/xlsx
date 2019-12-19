package xlsx

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io/ioutil"
	"os"
	"strings"

	jsoniter "github.com/json-iterator/go"

	"github.com/peterbourgon/diskv"
)

const (
	TRUE  = 0x01
	FALSE = 0x00
	US    = 0x1f // Unit Separator
	RS    = 0x1e // Record Separator
)

var (
	CellCacheSize uint64 = 1024 * 1024 // 1 MB per sheet

	json = jsoniter.ConfigCompatibleWithStandardLibrary
)

// DiskVCellStore is an implementation of the CellStore interface, backed by DiskV
type DiskVCellStore struct {
	baseDir string
	ibuf    []byte
	buf     *bytes.Buffer
	reader  *bytes.Reader
	store   *diskv.Diskv
	// enc     *gob.Encoder
	// dec     *gob.Decoder
	enc *jsoniter.Encoder
	dec *jsoniter.Decoder
}

func NewDiskVCellStore() (CellStore, error) {
	cs := &DiskVCellStore{
		buf: bytes.NewBuffer([]byte{}),
	}
	dir, err := ioutil.TempDir("", "cellstore")
	if err != nil {
		return nil, err
	}
	cs.baseDir = dir
	cs.store = diskv.New(diskv.Options{
		BasePath: dir,
		// Transform:    cellTransform,
		CacheSizeMax: CellCacheSize,
	})
	cs.enc = jsoniter.NewEncoder(cs.buf)
	cs.dec = jsoniter.NewDecoder(cs.buf)
	cs.ibuf = make([]byte, binary.MaxVarintLen64)
	return cs, nil
}

func (cs *DiskVCellStore) writeBool(b bool) error {
	if b {
		err := cs.buf.WriteByte(TRUE)
		if err != nil {
			return err
		}
	} else {
		err := cs.buf.WriteByte(FALSE)
		if err != nil {
			return err
		}
	}
	return cs.writeUnitSeparator()
}

//
func (cs *DiskVCellStore) writeUnitSeparator() error {
	return cs.buf.WriteByte(US)
}

//
func (cs *DiskVCellStore) readUnitSeparator() error {
	us, err := cs.reader.ReadByte()
	if err != nil {
		return err
	}
	if us != US {
		return errors.New("Invalid format in cellstore, not unit separator found")
	}
	return nil
}

//
func (cs *DiskVCellStore) readBool() (bool, error) {
	b, err := cs.reader.ReadByte()
	if err != nil {
		return false, err
	}
	err = cs.readUnitSeparator()
	if err != nil {
		return false, err
	}
	if b == TRUE {
		return true, nil
	}
	return false, nil
}

//-
func (cs *DiskVCellStore) writeString(s string) error {
	_, err := cs.buf.WriteString(s)
	if err != nil {
		return err
	}
	return cs.writeUnitSeparator()
}

//
func (cs *DiskVCellStore) readString() (string, error) {
	var s strings.Builder
	for {
		b, err := cs.reader.ReadByte()
		if err != nil {
			return "", err
		}
		if b == US {
			return s.String(), nil
		}
		err = s.WriteByte(b)
		if err != nil {
			return s.String(), err
		}
	}
	return s.String(), errors.New("This should be unreachable")
}

//
func (cs *DiskVCellStore) writeInt(i int) error {
	n := binary.PutVarint(cs.ibuf, int64(i))
	_, err := cs.buf.Write(cs.ibuf[:n])
	if err != nil {
		return err
	}
	return cs.writeUnitSeparator()
}

//
func (cs *DiskVCellStore) readInt() (int, error) {
	i, err := binary.ReadVarint(cs.reader)
	if err != nil {
		return -1, err
	}
	err = cs.readUnitSeparator()
	if err != nil {
		return -1, err
	}
	return int(i), nil
}

//
func (cs *DiskVCellStore) writeStringPointer(sp *string) error {
	err := cs.writeBool(sp == nil)
	if err != nil {
		return err
	}
	if sp != nil {
		_, err = cs.buf.WriteString(*sp)
		if err != nil {
			return err
		}
	}
	return cs.writeUnitSeparator()
}

//
func (cs *DiskVCellStore) readStringPointer() (*string, error) {
	isNil, err := cs.readBool()
	if err != nil {
		return nil, err
	}
	if isNil {
		err := cs.readUnitSeparator()
		return nil, err
	}
	s, err := cs.readString()
	return &s, err
}

//
func (cs *DiskVCellStore) writeEndOfRecord() error {
	return cs.buf.WriteByte(RS)
}

func (cs *DiskVCellStore) readEndOfRecord() error {
	b, err := cs.reader.ReadByte()
	if err != nil {
		return err
	}
	if b != RS {
		return errors.New("Expected end of record, but not found")
	}
	return nil
}

func (cs *DiskVCellStore) writeBorder(b Border) error {
	if err := cs.writeString(b.Left); err != nil {
		return err
	}
	if err := cs.writeString(b.LeftColor); err != nil {
		return err
	}
	if err := cs.writeString(b.Right); err != nil {
		return err
	}
	if err := cs.writeString(b.RightColor); err != nil {
		return err
	}
	if err := cs.writeString(b.Top); err != nil {
		return err
	}
	if err := cs.writeString(b.TopColor); err != nil {
		return err
	}
	if err := cs.writeString(b.Bottom); err != nil {
		return err
	}
	if err := cs.writeString(b.BottomColor); err != nil {
		return err
	}
	return nil
}

//
func (cs *DiskVCellStore) readBorder() (Border, error) {
	var err error
	b := Border{}
	if b.Left, err = cs.readString(); err != nil {
		return b, err
	}
	if b.LeftColor, err = cs.readString(); err != nil {
		return b, err
	}
	if b.Right, err = cs.readString(); err != nil {
		return b, err
	}
	if b.RightColor, err = cs.readString(); err != nil {
		return b, err
	}
	if b.Top, err = cs.readString(); err != nil {
		return b, err
	}
	if b.TopColor, err = cs.readString(); err != nil {
		return b, err
	}
	if b.Bottom, err = cs.readString(); err != nil {
		return b, err
	}
	if b.BottomColor, err = cs.readString(); err != nil {
		return b, err
	}
	return b, nil
}

func (cs *DiskVCellStore) writeFill(f Fill) error {
	if err := cs.writeString(f.PatternType); err != nil {
		return err
	}
	if err := cs.writeString(f.BgColor); err != nil {
		return err
	}
	if err := cs.writeString(f.FgColor); err != nil {
		return err
	}
	return nil
}

func (cs *DiskVCellStore) readFill() (Fill, error) {
	var err error
	f := Fill{}
	if f.PatternType, err = cs.readString(); err != nil {
		return f, err
	}
	if f.BgColor, err = cs.readString(); err != nil {
		return f, err
	}
	if f.FgColor, err = cs.readString(); err != nil {
		return f, err
	}
	return f, nil
}

func (cs *DiskVCellStore) writeFont(f Font) error {
	if err := cs.writeInt(f.Size); err != nil {
		return err
	}
	if err := cs.writeString(f.Name); err != nil {
		return err
	}
	if err := cs.writeInt(f.Family); err != nil {
		return err
	}
	if err := cs.writeInt(f.Charset); err != nil {
		return err
	}
	if err := cs.writeString(f.Color); err != nil {
		return err
	}
	if err := cs.writeBool(f.Bold); err != nil {
		return err
	}
	if err := cs.writeBool(f.Italic); err != nil {
		return err
	}
	if err := cs.writeBool(f.Underline); err != nil {
		return err
	}
	return nil
}

func (cs *DiskVCellStore) readFont() (Font, error) {
	var err error
	f := Font{}
	if f.Size, err = cs.readInt(); err != nil {
		return f, err
	}
	if f.Name, err = cs.readString(); err != nil {
		return f, err
	}
	if f.Family, err = cs.readInt(); err != nil {
		return f, err
	}
	if f.Charset, err = cs.readInt(); err != nil {
		return f, err
	}
	if f.Color, err = cs.readString(); err != nil {
		return f, err
	}
	if f.Bold, err = cs.readBool(); err != nil {
		return f, err
	}
	if f.Italic, err = cs.readBool(); err != nil {
		return f, err
	}
	if f.Underline, err = cs.readBool(); err != nil {
		return f, err
	}
	return f, nil
}

//
func (cs *DiskVCellStore) writeAlignment(a Alignment) error {
	var err error
	if err = cs.writeString(a.Horizontal); err != nil {
		return err
	}
	if err = cs.writeInt(a.Indent); err != nil {
		return err
	}
	if err = cs.writeBool(a.ShrinkToFit); err != nil {
		return err
	}
	if err = cs.writeInt(a.TextRotation); err != nil {
		return err
	}
	if err = cs.writeString(a.Vertical); err != nil {
		return err
	}
	if err = cs.writeBool(a.WrapText); err != nil {
		return err
	}
	return nil
}

func (cs *DiskVCellStore) readAlignment() (Alignment, error) {
	var err error
	a := Alignment{}
	if a.Horizontal, err = cs.readString(); err != nil {
		return a, err
	}
	if a.Indent, err = cs.readInt(); err != nil {
		return a, err
	}
	if a.ShrinkToFit, err = cs.readBool(); err != nil {
		return a, err
	}
	if a.TextRotation, err = cs.readInt(); err != nil {
		return a, err
	}
	if a.Vertical, err = cs.readString(); err != nil {
		return a, err
	}
	if a.WrapText, err = cs.readBool(); err != nil {
		return a, err
	}
	return a, nil
}

func (cs *DiskVCellStore) writeStyle(s *Style) error {
	var err error
	if err = cs.writeBorder(s.Border); err != nil {
		return err
	}
	if err = cs.writeFill(s.Fill); err != nil {
		return err
	}
	if err = cs.writeFont(s.Font); err != nil {
		return err
	}
	if err = cs.writeAlignment(s.Alignment); err != nil {
		return err
	}
	if err = cs.writeBool(s.ApplyBorder); err != nil {
		return err
	}
	if err = cs.writeBool(s.ApplyFill); err != nil {
		return err
	}
	if err = cs.writeBool(s.ApplyFont); err != nil {
		return err
	}
	if err = cs.writeBool(s.ApplyAlignment); err != nil {
		return err
	}
	if err = cs.writeEndOfRecord(); err != nil {
		return err
	}
	return nil
}

func (cs *DiskVCellStore) readStyle() (*Style, error) {
	var err error
	s := &Style{}
	if s.Border, err = cs.readBorder(); err != nil {
		return s, err
	}
	if s.Fill, err = cs.readFill(); err != nil {
		return s, err
	}
	if s.Font, err = cs.readFont(); err != nil {
		return s, err
	}
	if s.Alignment, err = cs.readAlignment(); err != nil {
		return s, err
	}
	if s.ApplyBorder, err = cs.readBool(); err != nil {
		return s, err
	}
	if s.ApplyFill, err = cs.readBool(); err != nil {
		return s, err
	}
	if s.ApplyFont, err = cs.readBool(); err != nil {
		return s, err
	}
	if s.ApplyAlignment, err = cs.readBool(); err != nil {
		return s, err
	}
	if err = cs.readEndOfRecord(); err != nil {
		return s, err
	}
	return s, nil
}

func (cs *DiskVCellStore) writeDataValidation(dv *xlsxDataValidation) error {
	var err error
	if err = cs.writeBool(dv.AllowBlank); err != nil {
		return err
	}
	if err = cs.writeBool(dv.ShowInputMessage); err != nil {
		return err
	}
	if err = cs.writeBool(dv.ShowErrorMessage); err != nil {
		return err
	}
	if err = cs.writeStringPointer(dv.ErrorStyle); err != nil {
		return err
	}
	if err = cs.writeStringPointer(dv.ErrorTitle); err != nil {
		return err
	}
	if err = cs.writeString(dv.Operator); err != nil {
		return err
	}
	if err = cs.writeStringPointer(dv.Error); err != nil {
		return err
	}
	if err = cs.writeStringPointer(dv.PromptTitle); err != nil {
		return err
	}
	if err = cs.writeStringPointer(dv.Prompt); err != nil {
		return err
	}
	if err = cs.writeString(dv.Type); err != nil {
		return err
	}
	if err = cs.writeString(dv.Sqref); err != nil {
		return err
	}
	if err = cs.writeString(dv.Formula1); err != nil {
		return err
	}
	if err = cs.writeString(dv.Formula2); err != nil {
		return err
	}
	if err = cs.writeEndOfRecord(); err != nil {
		return err
	}
	return nil
}

func (cs *DiskVCellStore) readDataValidation() (*xlsxDataValidation, error) {
	var err error
	dv := &xlsxDataValidation{}
	if dv.AllowBlank, err = cs.readBool(); err != nil {
		return dv, err
	}
	if dv.ShowInputMessage, err = cs.readBool(); err != nil {
		return dv, err
	}
	if dv.ShowErrorMessage, err = cs.readBool(); err != nil {
		return dv, err
	}
	if dv.ErrorStyle, err = cs.readStringPointer(); err != nil {
		return dv, err
	}
	if dv.ErrorTitle, err = cs.readStringPointer(); err != nil {
		return dv, err
	}
	if dv.Operator, err = cs.readString(); err != nil {
		return dv, err
	}
	if dv.Error, err = cs.readStringPointer(); err != nil {
		return dv, err
	}
	if dv.PromptTitle, err = cs.readStringPointer(); err != nil {
		return dv, err
	}
	if dv.Prompt, err = cs.readStringPointer(); err != nil {
		return dv, err
	}
	if dv.Type, err = cs.readString(); err != nil {
		return dv, err
	}
	if dv.Sqref, err = cs.readString(); err != nil {
		return dv, err
	}
	if dv.Formula1, err = cs.readString(); err != nil {
		return dv, err
	}
	if dv.Formula2, err = cs.readString(); err != nil {
		return dv, err
	}
	if err = cs.readEndOfRecord(); err != nil {
		return dv, err
	}
	return dv, nil
}

func (cs *DiskVCellStore) writeCell(c *Cell) error {
	var err error
	if err = cs.writeString(c.Value); err != nil {
		return err
	}
	if err = cs.writeString(c.formula); err != nil {
		return err
	}
	if err = cs.writeBool(c.style != nil); err != nil {
		return err
	}
	if err = cs.writeString(c.NumFmt); err != nil {
		return err
	}
	if err = cs.writeBool(c.date1904); err != nil {
		return err
	}
	if err = cs.writeBool(c.Hidden); err != nil {
		return err
	}
	if err = cs.writeInt(c.HMerge); err != nil {
		return err
	}
	if err = cs.writeInt(c.VMerge); err != nil {
		return err
	}
	if err = cs.writeInt(int(c.cellType)); err != nil {
		return err
	}
	if err = cs.writeBool(c.DataValidation != nil); err != nil {
		return err
	}
	if err = cs.writeString(c.Hyperlink.DisplayString); err != nil {
		return err
	}
	if err = cs.writeString(c.Hyperlink.Link); err != nil {
		return err
	}
	if err = cs.writeString(c.Hyperlink.Tooltip); err != nil {
		return err
	}
	if err = cs.writeInt(c.num); err != nil {
		return err
	}
	if err = cs.writeEndOfRecord(); err != nil {
		return err
	}
	if c.style != nil {
		if err = cs.writeStyle(c.style); err != nil {
			return err
		}
	}
	if c.DataValidation != nil {
		if err = cs.writeDataValidation(c.DataValidation); err != nil {
			return err
		}
	}
	return nil
}

//
func (cs *DiskVCellStore) readCell() (*Cell, error) {
	var err error
	var cellType int
	var hasStyle, hasDataValidation bool
	c := &Cell{}
	if c.Value, err = cs.readString(); err != nil {
		return c, err
	}
	if c.formula, err = cs.readString(); err != nil {
		return c, err
	}
	if hasStyle, err = cs.readBool(); err != nil {
		return c, err
	}
	if c.NumFmt, err = cs.readString(); err != nil {
		return c, err
	}
	if c.date1904, err = cs.readBool(); err != nil {
		return c, err
	}
	if c.Hidden, err = cs.readBool(); err != nil {
		return c, err
	}
	if c.HMerge, err = cs.readInt(); err != nil {
		return c, err
	}
	if c.VMerge, err = cs.readInt(); err != nil {
		return c, err
	}
	if cellType, err = cs.readInt(); err != nil {
		return c, err
	}
	c.cellType = CellType(cellType)
	if hasDataValidation, err = cs.readBool(); err != nil {
		return c, err
	}
	if c.Hyperlink.DisplayString, err = cs.readString(); err != nil {
		return c, err
	}
	if c.Hyperlink.Link, err = cs.readString(); err != nil {
		return c, err
	}
	if c.Hyperlink.Tooltip, err = cs.readString(); err != nil {
		return c, err
	}
	if c.num, err = cs.readInt(); err != nil {
		return c, err
	}
	if err = cs.readEndOfRecord(); err != nil {
		return c, err
	}
	if hasStyle {
		if c.style, err = cs.readStyle(); err != nil {
			return c, err
		}
	}
	if hasDataValidation {
		if c.DataValidation, err = cs.readDataValidation(); err != nil {
			return c, err
		}
	}
	return c, nil
}

func (cs *DiskVCellStore) WriteCell(c *Cell) error {
	cs.buf.Reset()
	err := cs.writeCell(c)
	if err != nil {
		return err
	}
	key := c.key()
	return cs.store.WriteStream(key, cs.buf, true)

}

func (cs *DiskVCellStore) ReadCell(key string) (*Cell, error) {
	b, err := cs.store.Read(key)
	if err != nil {
		if _, ok := err.(*os.PathError); ok {
			return nil, NewCellNotFoundError(key, err.Error())
		}
		return nil, err
	}
	cs.buf.Reset()
	_, err = cs.buf.Write(b)
	if err != nil {
		return nil, err
	}
	cs.reader = bytes.NewReader(cs.buf.Bytes())
	return cs.readCell()
}

//
func (cs *DiskVCellStore) DeleteCell(key string) error {
	return cs.store.Erase(key)
}

//
func (cs *DiskVCellStore) ForEach(cvf CellVisitorFunc) error {
	for key := range cs.store.Keys(nil) {
		c, err := cs.ReadCell(key)
		if err != nil {
			return err
		}
		err = cvf(c)
		if err != nil {
			return err
		}
	}
	return nil
}

func (cs *DiskVCellStore) ForEachInRow(r *Row, cvf CellVisitorFunc) error {
	pref := r.makeCellKeyRowPrefix()
	for key := range cs.store.KeysPrefix(pref, nil) {
		c, err := cs.ReadCell(key)
		if err != nil {
			return err
		}
		err = cvf(c)
		if err != nil {
			return err
		}
	}
	return nil
}

//
func (cs *DiskVCellStore) Close() error {
	return os.RemoveAll(cs.baseDir)

}

func cellTransform(s string) []string {
	return strings.Split(s, ":")
}
