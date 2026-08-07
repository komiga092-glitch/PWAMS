package constants

const (
    ErrAuthenticationRequired      = "Authentication required"
    ErrInvalidAuthContext          = "Invalid authentication context"
    ErrInvalidPersonID             = "Invalid person ID"
    ErrInvalidAidRequestID         = "Invalid aid request ID"

    ErrInvalidAidRequestInfo       = "Invalid aid request information"
    ErrAidRequestNotFound          = "Aid request not found"
    ErrPersonNotFound              = "Person not found"
    ErrInvalidReviewInfo           = "Invalid review information"
    ErrCancellationReasonRequired  = "Cancellation reason is required"
    ErrAidRequestCannotBeDeleted   = "Aid request cannot be deleted"
	ErrInvalidRoleAssignment = "Only Super Admin can assign the Super Admin role"
)