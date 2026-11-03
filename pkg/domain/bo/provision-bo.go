package bo

// Person is a user to provision.
type Person struct{ Email, Name string }

// ProvisionOptions describes an owner/viewer pair and their shared space.
type ProvisionOptions struct {
	Owner, Viewer Person
	Timezone      string
	SpaceName     string
}
