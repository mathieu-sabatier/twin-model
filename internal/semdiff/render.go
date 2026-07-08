package semdiff

import (
	"fmt"
	"strings"
)

// render builds the human-readable one-liner for a Change. It is the single
// source of the Text field, shared by the UI and the PR comment.
func render(c Change) string {
	switch c.Kind {
	case TypeAdded:
		return fmt.Sprintf("Added type %s", c.Type)
	case TypeRemoved:
		return fmt.Sprintf("Removed type %s", c.Type)
	case TypeChanged:
		return fmt.Sprintf("%s: %s %s → %s", c.Type, c.Field, orDash(c.Old), orDash(c.New))
	case MemberAdded:
		return fmt.Sprintf("%s: added member %s", c.Type, c.Member)
	case MemberRemoved:
		return fmt.Sprintf("%s: removed member %s", c.Type, c.Member)
	case MemberChanged:
		return fmt.Sprintf("%s.%s: %s %s → %s", c.Type, c.Member, c.Field, orDash(c.Old), orDash(c.New))
	case EnumAdded:
		return fmt.Sprintf("Added enum %s", c.Enum)
	case EnumRemoved:
		return fmt.Sprintf("Removed enum %s", c.Enum)
	case EnumChanged:
		return fmt.Sprintf("Enum %s: %s %s → %s", c.Enum, c.Field, orDash(c.Old), orDash(c.New))
	case EnumValueChanged:
		return fmt.Sprintf("Enum %s: %s → %s", c.Enum, orDash(c.Old), orDash(c.New))
	case InstanceAdded:
		return fmt.Sprintf("Added instance %s (%s) under %s", c.Instance, c.Type, c.Under)
	case InstanceRemoved:
		return fmt.Sprintf("Removed instance %s (%s)", c.Instance, c.Type)
	case ValueChanged:
		if c.New == "" {
			return fmt.Sprintf("%s.%s: cleared (was %q)", c.Instance, c.Member, c.Old)
		}
		return fmt.Sprintf("%s.%s: → %q", c.Instance, c.Member, c.New)
	case ChildAdded:
		return fmt.Sprintf("%s: added child %s", c.Instance, c.Child)
	case ChildRemoved:
		return fmt.Sprintf("%s: removed child %s", c.Instance, c.Child)
	case InstanceChanged:
		return fmt.Sprintf("%s: %s %s → %s", c.Instance, c.Field, orDash(c.Old), orDash(c.New))
	case ImportAdded:
		return fmt.Sprintf("+ import %s: %s", c.Field, c.New)
	case ImportRemoved:
		return fmt.Sprintf("- import %s: %s", c.Field, c.Old)
	case NamespaceChanged:
		return fmt.Sprintf("namespace %s → %s", orDash(c.Old), orDash(c.New))
	case VersionChanged:
		return fmt.Sprintf("version %s → %s", orDash(c.Old), orDash(c.New))
	default:
		return string(c.Kind)
	}
}

// Render joins the changelist into a bullet list for a PR comment.
func Render(changes []Change) string {
	if len(changes) == 0 {
		return "_No semantic changes._"
	}
	var b strings.Builder
	for _, c := range changes {
		b.WriteString("- ")
		b.WriteString(c.Text)
		b.WriteByte('\n')
	}
	return b.String()
}

func orDash(s string) string {
	if s == "" {
		return "∅"
	}
	return s
}
