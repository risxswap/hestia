package job

type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (s *Service) List() ListJobsResult {
	return ListJobsResult{Items: []JobSummary{}}
}
