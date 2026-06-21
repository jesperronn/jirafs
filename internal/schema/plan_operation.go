package schema

// OperationType represents the kind of plan operation applied to an editable
// field.
type OperationType string

const (
	OpSet    OperationType = "set"
	OpAppend OperationType = "append"
	OpRemove OperationType = "remove"
)

// ValidOperationTypes returns the set of all recognized operation types.
var ValidOperationTypes = []OperationType{
	OpSet,
	OpAppend,
	OpRemove,
}

// IsValidOperationType reports whether ot is a known operation type.
func IsValidOperationType(ot OperationType) bool {
	for _, v := range ValidOperationTypes {
		if v == ot {
			return true
		}
	}
	return false
}

// PlanOperation represents a typed plan operation targeting one editable field.
type PlanOperation struct {
	// Field is the editable field this operation targets.
	Field EditableField
	// Type is the kind of operation (set, append, remove).
	Type OperationType
	// Value is the value to apply. For append/remove operations
	// it is the item to add or remove; for set it is the new value.
	Value string
}

// IsZero reports whether o is the zero value.
func (o PlanOperation) IsZero() bool {
	return o.Field == "" && o.Type == "" && o.Value == ""
}

// String renders the operation as a compact "type:field:value" string.
func (o PlanOperation) String() string {
	return string(o.Type) + ":" + string(o.Field) + ":" + o.Value
}

// Equals reports whether o and p represent the same operation.
func (o PlanOperation) Equals(p PlanOperation) bool {
	return o.Field == p.Field && o.Type == p.Type && o.Value == p.Value
}
