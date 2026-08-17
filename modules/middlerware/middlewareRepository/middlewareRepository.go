package middlewareRepository

type (
	MiddlewareRepositoryService interface{}

	middlewarerepository struct{}
)

func NewMiddlewarerepository() MiddlewareRepositoryService {
	return &middlewarerepository{}
}
