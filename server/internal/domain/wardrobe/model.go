package wardrobe

type Item struct {
	ID         int64
	PublicID   string
	UserID     int64
	Name       string
	Category   string
	Color      string
	Silhouette string
	Material   string
	Season     string
	UserNotes  string
	IsCore     bool
	Status     string
}

type Input struct {
	Name       string
	Category   string
	Color      string
	Silhouette string
	Material   string
	Season     string
	Notes      string
}
