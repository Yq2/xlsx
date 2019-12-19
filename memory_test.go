package xlsx

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestMemoryCellStore(t *testing.T) {
	c := qt.New(t)

	c.Run("CellNotFoundError", func(c *qt.C) {
		memoryCs, err := NewMemoryCellStore()
		c.Assert(err, qt.IsNil)
		cs, ok := memoryCs.(*MemoryCellStore)
		c.Assert(ok, qt.Equals, true)
		defer cs.Close()

		cell, err := cs.ReadCell("I don't exist")
		c.Assert(err, qt.Not(qt.IsNil))
		c.Assert(cell, qt.IsNil)
		_, ok = err.(*CellNotFoundError)
		c.Assert(ok, qt.Equals, true)
	})

	c.Run("Write and Read Cell", func(c *qt.C) {
		mCs, err := NewMemoryCellStore()
		c.Assert(err, qt.IsNil)
		cs, ok := mCs.(*MemoryCellStore)
		c.Assert(ok, qt.Equals, true)
		defer cs.Close()

		s := &Style{
			Border: Border{
				Left:        "left",
				LeftColor:   "leftColor",
				Right:       "right",
				RightColor:  "rightColor",
				Top:         "top",
				TopColor:    "topColor",
				Bottom:      "bottom",
				BottomColor: "bottomColor",
			},
			Fill: Fill{
				PatternType: "PatternType",
				BgColor:     "BgColor",
				FgColor:     "FgColor",
			},
			Font: Font{
				Size:      1,
				Name:      "Font",
				Family:    2,
				Charset:   3,
				Color:     "Red",
				Bold:      true,
				Italic:    true,
				Underline: true,
			},
			Alignment: Alignment{
				Horizontal:   "left",
				Indent:       1,
				ShrinkToFit:  true,
				TextRotation: 90,
				Vertical:     "top",
				WrapText:     true,
			},
			ApplyBorder:    true,
			ApplyFill:      true,
			ApplyFont:      true,
			ApplyAlignment: true,
		}

		dv := &xlsxDataValidation{
			AllowBlank:       true,
			ShowInputMessage: true,
			ShowErrorMessage: true,
			Type:             "type",
			Sqref:            "sqref",
			Formula1:         "formula1",
			Formula2:         "formula1",
			Operator:         "operator",
		}

		file := NewFile()
		sheet, _ := file.AddSheet("Test")
		row := sheet.AddRow()
		cell := row.AddCell()

		cell.Value = "value"
		cell.formula = "formula"
		cell.style = s
		cell.NumFmt = "numFmt"
		cell.date1904 = true
		cell.Hidden = true
		cell.HMerge = 49
		cell.VMerge = 50
		cell.cellType = CellType(2)
		cell.DataValidation = dv
		cell.Hyperlink = Hyperlink{
			DisplayString: "displaystring",
			Link:          "link",
			Tooltip:       "tooltip",
		}

		err = cs.WriteCell(cell)
		c.Assert(err, qt.IsNil)
		cell2, err := cs.ReadCell(cell.key())
		c.Assert(err, qt.IsNil)
		c.Assert(cell.Value, qt.Equals, cell2.Value)
		c.Assert(cell.formula, qt.Equals, cell2.formula)
		c.Assert(cell.NumFmt, qt.Equals, cell2.NumFmt)
		c.Assert(cell.date1904, qt.Equals, cell2.date1904)
		c.Assert(cell.Hidden, qt.Equals, cell2.Hidden)
		c.Assert(cell.HMerge, qt.Equals, cell2.HMerge)
		c.Assert(cell.VMerge, qt.Equals, cell2.VMerge)
		c.Assert(cell.cellType, qt.Equals, cell2.cellType)
		c.Assert(*cell.DataValidation, qt.DeepEquals, *cell2.DataValidation)
		c.Assert(cell.Hyperlink, qt.DeepEquals, cell2.Hyperlink)
		c.Assert(cell.num, qt.Equals, cell2.num)

		s2 := cell2.style
		c.Assert(s2.Border, qt.DeepEquals, s.Border)
		c.Assert(s2.Fill, qt.DeepEquals, s.Fill)
		c.Assert(s2.Font, qt.DeepEquals, s.Font)
		c.Assert(s2.Alignment, qt.DeepEquals, s.Alignment)
		c.Assert(s2.ApplyBorder, qt.Equals, s.ApplyBorder)
		c.Assert(s2.ApplyFill, qt.Equals, s.ApplyFill)
		c.Assert(s2.ApplyFont, qt.Equals, s.ApplyFont)
		c.Assert(s2.ApplyAlignment, qt.Equals, s.ApplyAlignment)

	})

	c.Run("Delete Cell", func(c *qt.C) {
		memoryCs, err := NewMemoryCellStore()
		c.Assert(err, qt.IsNil)
		cs, ok := memoryCs.(*MemoryCellStore)
		c.Assert(ok, qt.Equals, true)
		defer cs.Close()

		file := NewFile()
		sheet, _ := file.AddSheet("Test")
		row := sheet.AddRow()
		cell := row.AddCell()

		err = cs.WriteCell(cell)
		c.Assert(err, qt.IsNil)
		_, err = cs.ReadCell(cell.key())
		c.Assert(err, qt.IsNil)
		err = cs.DeleteCell(cell.key())
		c.Assert(err, qt.IsNil)
		_, err = cs.ReadCell(cell.key())
		c.Assert(err, qt.Not(qt.IsNil))
	})

}
