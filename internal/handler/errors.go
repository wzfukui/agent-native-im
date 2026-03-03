package handler

// Error codes — machine-readable identifiers for every error class.
// Frontend and LLM can match on these codes for programmatic handling.
const (
	// ── Auth ──
	ErrCodeAuthRequired       = "AUTH_REQUIRED"
	ErrCodeAuthInvalid        = "AUTH_INVALID_CREDENTIALS"
	ErrCodeAuthTokenExpired   = "AUTH_TOKEN_EXPIRED"
	ErrCodeAuthTokenGenFailed = "AUTH_TOKEN_GEN_FAILED"
	ErrCodeAuthBootstrapOnly  = "AUTH_BOOTSTRAP_ONLY"

	// ── Permission ──
	ErrCodePermDenied        = "PERM_DENIED"
	ErrCodePermNotOwner      = "PERM_NOT_OWNER"
	ErrCodePermNotParticipant = "PERM_NOT_PARTICIPANT"
	ErrCodePermNotAdmin      = "PERM_NOT_ADMIN"
	ErrCodePermObserver      = "PERM_OBSERVER_RESTRICTED"

	// ── Validation ──
	ErrCodeValidation       = "VALIDATION_ERROR"
	ErrCodeValidationField  = "VALIDATION_FIELD_INVALID"
	ErrCodeValidationFormat = "VALIDATION_FORMAT_ERROR"

	// ── Not Found ──
	ErrCodeNotFound         = "NOT_FOUND"
	ErrCodeEntityNotFound   = "ENTITY_NOT_FOUND"
	ErrCodeMessageNotFound  = "MESSAGE_NOT_FOUND"
	ErrCodeConvNotFound     = "CONVERSATION_NOT_FOUND"
	ErrCodeTaskNotFound     = "TASK_NOT_FOUND"
	ErrCodeInviteNotFound   = "INVITE_NOT_FOUND"
	ErrCodeWebhookNotFound  = "WEBHOOK_NOT_FOUND"
	ErrCodeDeviceNotFound   = "DEVICE_NOT_FOUND"

	// ── Conflict ──
	ErrCodeConflict         = "CONFLICT"
	ErrCodeDuplicateName    = "CONFLICT_DUPLICATE_NAME"
	ErrCodeDuplicateUser    = "CONFLICT_DUPLICATE_USER"
	ErrCodeAlreadyMember    = "CONFLICT_ALREADY_MEMBER"
	ErrCodeAlreadyRevoked   = "CONFLICT_ALREADY_REVOKED"
	ErrCodeAlreadyResolved  = "CONFLICT_ALREADY_RESOLVED"

	// ── State ──
	ErrCodeStateBadTransition = "STATE_BAD_TRANSITION"
	ErrCodeStateExpired       = "STATE_EXPIRED"
	ErrCodeStateLimitReached  = "STATE_LIMIT_REACHED"

	// ── Internal ──
	ErrCodeInternal    = "INTERNAL_ERROR"
	ErrCodeDBError     = "INTERNAL_DB_ERROR"
	ErrCodeFileError   = "INTERNAL_FILE_ERROR"
	ErrCodePushError   = "INTERNAL_PUSH_ERROR"
	ErrCodeConfigError = "INTERNAL_CONFIG_ERROR"
)
