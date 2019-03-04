package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/emicklei/proto"
)

type Comparer struct {
	Older map[string]*proto.Message
	Newer map[string]*proto.Message
}

var cmp Comparer

type Condition int

const (
	ChangedLabel            Condition = 1
	AddedField              Condition = 2
	RemovedField            Condition = 3
	ChangedName             Condition = 4
	ChangedType             Condition = 5
	ChangedNumber           Condition = 6
	ChangedDefault          Condition = 7
	ChangedTypeName         Condition = 8
	NonFieldIncompatibility Condition = 9
)

type Difference struct {
	condition Condition
	newValue  string
	oldValue  string
	path      string
	qualifier string
	message   string
}

func (d *Difference) String() string {
	path := ""
	if d.path == "" {
		path = "."
	} else {
		path = d.path
	}
	if d.condition == ChangedLabel {
		return "Changed label of field nr " + d.qualifier + " in " + path + " from " + d.oldValue + " to " + d.newValue
	} else if d.condition == AddedField {
		return "Added Field " + d.qualifier + " in " + path + " of label " + d.newValue + d.message
	} else if d.condition == RemovedField {
		return "Removed Field " + d.qualifier + " in " + path + " of label " + d.newValue + d.message
	} else if d.condition == ChangedName {
		return "Changed name of field " + d.qualifier + " in " + path + " from " + d.oldValue + " to " + d.newValue
	} else if d.condition == ChangedType {
		return "Changed type of field " + d.qualifier + " in " + path + " from " + d.oldValue + " to " + d.newValue
	} else if d.condition == ChangedNumber {
		return "Changed numeric tag of field named \"" + d.qualifier + "\" in " + path + " from " + d.oldValue + " to " + d.newValue
	} else if d.condition == ChangedDefault {
		return "Changed default value of field " + d.qualifier + " in " + path + " from " + d.oldValue + " to " + d.newValue + " this is generally OK"
	} else if d.condition == NonFieldIncompatibility {
		return d.message
	} else if d.condition == ChangedTypeName {
		return "Changed TypeName of field " + d.qualifier + " from " + d.oldValue + " to " + d.newValue + " in " + path + " manually compare message types using compare message method"
	}
	return ""
}

type DifferenceList struct {
	Error     []Difference
	Warning   []Difference
	Extension []Difference
}

func (d *DifferenceList) String(suppressWarning bool) string {
	var output string = ""
	if !suppressWarning && d.Warning != nil {
		output = output + "WARNING\n"
		for _, val := range d.Warning {
			output = output + val.String() + "\n"
		}
	}
	if d.Error != nil {
		output = output + "INCOMPATIBILITIES\n"
		for _, val := range d.Error {
			output = output + val.String() + "\n"
		}
	}

	output = output + fmt.Sprintf("%d Warnings, %d Incompability Errors\n", len(d.Warning), len(d.Error))
	return output
}

func (d1 *DifferenceList) merge(d2 DifferenceList) {
	d1.Error = append(d1.Error, d2.Error...)
	d1.Warning = append(d1.Warning, d2.Warning...)
}

func (d *DifferenceList) addWarning(c Condition, newValue, oldValue, path, qualifier, message string) {
	d1 := Difference{c, newValue, oldValue, path, qualifier, message}
	d.Warning = append(d.Warning, d1)
}

func (d *DifferenceList) addError(c Condition, newValue, oldValue, path, qualifier, message string) {
	d1 := Difference{c, newValue, oldValue, path, qualifier, message}
	d.Error = append(d.Error, d1)
}

func Compare() DifferenceList {
	var output DifferenceList

	for m1_name, m1 := range cmp.Newer {
		exist := false
		for m2_name, m2 := range cmp.Older {
			if m1_name == m2_name {
				exist = true
				output.merge(compareMessageFields(m2, m1))
			}
		}
		if !exist {
			output.addWarning(NonFieldIncompatibility, "", "", "", "", "Added message "+m1_name)
		}
	}

	for m2_name, _ := range cmp.Older {
		exist := false
		for m1_name, _ := range cmp.Newer {
			if m1_name == m2_name {
				exist = true
			}
		}
		if !exist {
			output.addWarning(NonFieldIncompatibility, "", "", "", "", "Removed message "+m2_name)
		}
	}

	return output
}

func compareMessageFields(m_old *proto.Message, m_new *proto.Message) DifferenceList {
	var output DifferenceList
	path := m_old.Name

	for _, e1 := range m_new.Elements {
		exist := false
		for _, e2 := range m_old.Elements {
			field_1, ok_1 := e1.(*proto.NormalField)
			field_2, ok_2 := e2.(*proto.NormalField)
			if ok_1 && ok_2 {
				if field_1.Sequence == field_2.Sequence {
					exist = true
					output.merge(compareFields(field_1, field_2, path))

				}
			}
		}
		if !exist {
			field, ok := e1.(*proto.NormalField)
			if ok {
				if field.Required == true {
					output.addError(AddedField, "Required", "", path, strconv.Itoa(int(field.Sequence)), "")
				} else if field.Optional == true {
					output.addWarning(AddedField, "Optional", "", path, strconv.Itoa(int(field.Sequence)), "")
				}
			}
		}
	}

	for _, e2 := range m_old.Elements {
		exist := false
		for _, e1 := range m_new.Elements {
			field_1, ok_1 := e1.(*proto.NormalField)
			field_2, ok_2 := e2.(*proto.NormalField)
			if ok_1 && ok_2 {
				if field_1.Sequence == field_2.Sequence {
					exist = true
				}
			}
		}
		if !exist {
			field, ok := e2.(*proto.NormalField)
			if ok {
				if field.Required == true {
					output.addError(RemovedField, "Required", "", path, strconv.Itoa(int(field.Sequence)), "")
				} else {
					output.addWarning(RemovedField, "Optional", "", path, strconv.Itoa(int(field.Sequence)), "")
				}
			}
		}
	}

	for _, e1 := range m_new.Elements {
		for _, e2 := range m_old.Elements {
			field_1, ok_1 := e1.(*proto.NormalField)
			field_2, ok_2 := e2.(*proto.NormalField)
			if ok_1 && ok_2 {
				if field_1.Name == field_2.Name && field_1.Sequence != field_2.Sequence {
					output.addError(ChangedNumber, strconv.Itoa(int(field_1.Sequence)), strconv.Itoa(int(field_2.Sequence)), path, field_1.Name, "Semantics may be changed for this")
				}
			}
		}
	}

	return output
}

func compareFields(f_old, f_new *proto.NormalField, path string) DifferenceList {
	var output DifferenceList
	var f_old_label, f_new_label string
	if f_old.Required {
		f_old_label = "Required"
	} else if f_old.Optional {
		f_old_label = "Optional"
	} else if f_old.Repeated {
		f_old_label = "Repeated"
	}

	if f_new.Required {
		f_new_label = "Required"
	} else if f_old.Optional {
		f_new_label = "Optional"
	} else if f_old.Repeated {
		f_new_label = "Repeated"
	}

	if f_old.Required != f_new.Required || f_old.Optional != f_new.Optional || f_old.Repeated != f_new.Repeated {
		output.addWarning(ChangedLabel, f_old_label, f_new_label, path, strconv.Itoa(int(f_old.Sequence)), "")
	}

	if f_old.Name != f_new.Name {
		output.addWarning(ChangedName, f_old.Name, f_new.Name, path, strconv.Itoa(int(f_old.Sequence)), "")
	}

	if f_old.Type != f_new.Type {
		output.addWarning(ChangedType, f_old.Type, f_new.Type, path, strconv.Itoa(int(f_old.Sequence)), "")
	}

	return output
}

func handleOldMessage(m *proto.Message) {
	cmp.Older[m.Name] = m
}

func handleNewMessage(m *proto.Message) {
	cmp.Newer[m.Name] = m
}

func main() {
	if len(os.Args) == 3 {
		older, _ := os.Open(os.Args[1])
		defer older.Close()
		newer, _ := os.Open(os.Args[2])
		defer newer.Close()

		parser_old := proto.NewParser(older)
		definition_old, _ := parser_old.Parse()
		parser_new := proto.NewParser(newer)
		definition_new, _ := parser_new.Parse()

		cmp.Older = make(map[string]*proto.Message)
		cmp.Newer = make(map[string]*proto.Message)

		proto.Walk(definition_old,
			proto.WithMessage(handleOldMessage))
		proto.Walk(definition_new,
			proto.WithMessage(handleNewMessage))

		output := Compare()
		fmt.Print(output.String(false))
	} else {
		fmt.Println("Please input like this ./comp old.proto new.proto")
	}
}
