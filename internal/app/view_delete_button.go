package app

// DeleteButtonView holds data for the delete button template fragment
type DeleteButtonView struct {
	URL            string // e.g., "/categories/abc123"
	ConfirmMessage string // e.g., "Delete this category and all its projects?"
	ButtonText     string // e.g., "Delete Category"
}
