package params

// APIErrorResponse holds information about an error, returned by the API
type APIErrorResponse struct {
	Error   string `json:"error"`
	Details string `json:"details"`
}

var (
	// NotFoundResponse is returned when a resource is not found
	NotFoundResponse = APIErrorResponse{
		Error:   "Not Found",
		Details: "The resource you are looking for was not found",
	}
	// UnauthorizedResponse is a canned response for unauthorized access
	UnauthorizedResponse = APIErrorResponse{
		Error:   "Not Authorized",
		Details: "You do not have the required permissions to access this resource",
	}
	// InitializationRequired is returned if gopherbin has not beed properly initialized
	InitializationRequired = APIErrorResponse{
		Error:   "init_required",
		Details: "Missing superuser",
	}
)
