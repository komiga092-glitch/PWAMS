package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/constants"
	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/services"
)

// SelfServiceHandler handles Beneficiary and Student self-service operations.
// All operations derive the person identity from the authenticated user's
// email — client-supplied person IDs are never trusted.
type SelfServiceHandler struct {
	aidRequestService    *services.AidRequestService
	loanService          *services.LoanService
	loanRepaymentService *services.LoanRepaymentService
	careService          *services.CareProvidedService
	dashboardService     *services.DashboardService
	auditLogService      *services.AuditLogService
	personRepo           *repository.PersonRepository
	notificationService  *services.NotificationService
}

// NewSelfServiceHandler creates a new SelfServiceHandler.
func NewSelfServiceHandler(
	aidRequestService *services.AidRequestService,
	loanService *services.LoanService,
	loanRepaymentService *services.LoanRepaymentService,
	careService *services.CareProvidedService,
	dashboardService *services.DashboardService,
	auditLogService *services.AuditLogService,
	personRepo *repository.PersonRepository,
	notificationService *services.NotificationService,
) *SelfServiceHandler {
	return &SelfServiceHandler{
		aidRequestService:    aidRequestService,
		loanService:          loanService,
		loanRepaymentService: loanRepaymentService,
		careService:          careService,
		dashboardService:     dashboardService,
		auditLogService:      auditLogService,
		personRepo:           personRepo,
		notificationService:  notificationService,
	}
}

// getCurrentUser returns the authenticated user from the Gin context.
func (h *SelfServiceHandler) getCurrentUser(c *gin.Context) (*models.User, bool) {
	currentUserValue, exists := c.Get("current_user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": constants.ErrAuthenticationRequired,
		})
		return nil, false
	}

	currentUser, ok := currentUserValue.(*models.User)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": constants.ErrInvalidAuthContext,
		})
		return nil, false
	}

	return currentUser, true
}

// isBeneficiaryOrStudent checks if the user has Beneficiary or Student role.
func (h *SelfServiceHandler) isBeneficiaryOrStudent(user *models.User) bool {
	return user.Role.Name == models.RoleBeneficiary || user.Role.Name == models.RoleStudent
}

// getPersonForUser retrieves the person record linked to the user's email.
// Returns nil with no error if person not found (for graceful handling).
func (h *SelfServiceHandler) getPersonForUser(c *gin.Context) (*models.Person, bool) {
	user, ok := h.getCurrentUser(c)
	if !ok {
		return nil, false
	}

	person, err := h.personRepo.FindByEmail(user.Email)
	if err != nil {
		if errors.Is(err, repository.ErrPersonNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "No beneficiary profile linked to this account",
			})
			return nil, false
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Unable to retrieve profile",
		})
		return nil, false
	}

	return person, true
}

// Dashboard renders the self-service dashboard.
func (h *SelfServiceHandler) Dashboard(c *gin.Context) {
	user, ok := h.getCurrentUser(c)
	if !ok {
		return
	}

	pageTemplate := "beneficiary_dashboard_content"
	if user.Role.Name == models.RoleStudent {
		pageTemplate = "student_dashboard_content"
	}

	data, err := h.dashboardService.GetSelfServiceDashboard(
		user.Email,
		h.personRepo,
		h.aidRequestService,
		h.loanService,
		h.loanRepaymentService,
	)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "base", PageData(c, gin.H{
			"title":         "Dashboard",
			"page_template": pageTemplate,
			"error":         "Unable to load dashboard data.",
		}))
		return
	}

	if h.notificationService != nil {
		if unread, err := h.notificationService.CountUnreadForUser(user.ID); err == nil {
			data.UnreadNotifications = unread
		}
	}

	c.HTML(http.StatusOK, "base", PageData(c, gin.H{
		"title":         "Dashboard",
		"page_template": pageTemplate,
		"data":          data,
	}))
}

// MyAid renders the user's aid requests list.
func (h *SelfServiceHandler) MyAid(c *gin.Context) {
	user, ok := h.getCurrentUser(c)
	if !ok {
		return
	}

	requests, err := h.aidRequestService.GetAidRequestsForUser(user.Email)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "base", PageData(c, gin.H{
			"title":         "My Aid",
			"page_template": "my_aid_content",
			"error":         "Unable to load aid requests.",
		}))
		return
	}

	c.HTML(http.StatusOK, "base", PageData(c, gin.H{
		"title":         "My Aid",
		"page_template": "my_aid_content",
		"requests":      requests,
		"submitted":     c.Query("submitted") == "1",
	}))
}

// RequestAid renders the self-service aid request form.
func (h *SelfServiceHandler) RequestAid(c *gin.Context) {
	_, ok := h.getCurrentUser(c)
	if !ok {
		return
	}

	c.HTML(http.StatusOK, "base", PageData(c, gin.H{
		"title":         "Request Aid",
		"page_template": "request_aid_content",
	}))
}

// RequestAidSubmit handles the aid request submission.
func (h *SelfServiceHandler) RequestAidSubmit(c *gin.Context) {
	user, ok := h.getCurrentUser(c)
	if !ok {
		return
	}

	var request models.SelfServiceAidRequest
	if err := c.ShouldBind(&request); err != nil {
		c.HTML(http.StatusUnprocessableEntity, "base", PageData(c, gin.H{
			"title":         "Request Aid",
			"page_template": "request_aid_content",
			"error":         "Please complete all required fields.",
		}))
		return
	}

	aidRequest, err := h.aidRequestService.CreateAidRequestForUser(user.ID, user.Email, request)
	if err != nil {
		message := "Unable to submit aid request."
		if errors.Is(err, services.ErrPersonNotLinked) {
			message = "No beneficiary profile is linked to this account."
		}
		c.HTML(http.StatusUnprocessableEntity, "base", PageData(c, gin.H{
			"title":         "Request Aid",
			"page_template": "request_aid_content",
			"error":         message,
		}))
		return
	}

	// Audit log
	if h.auditLogService != nil {
		_ = h.auditLogService.Create(
			user.ID.String(),
			"CREATE_AID_REQUEST",
			"aid_requests",
			aidRequest.ID.String(),
			"Self-service aid request submitted",
		)
	}

	c.Redirect(http.StatusSeeOther, "/my/aid?submitted=1")
}

// MyLoans renders the user's loans list.
func (h *SelfServiceHandler) MyLoans(c *gin.Context) {
	user, ok := h.getCurrentUser(c)
	if !ok {
		return
	}

	loans, err := h.loanService.GetLoansForUser(user.Email)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "base", PageData(c, gin.H{
			"title":         "My Loans",
			"page_template": "my_loans_content",
			"error":         "Unable to load loans.",
		}))
		return
	}

	c.HTML(http.StatusOK, "base", PageData(c, gin.H{
		"title":         "My Loans",
		"page_template": "my_loans_content",
		"loans":         loans,
		"submitted":     c.Query("submitted") == "1",
	}))
}

// ApplyForLoan renders the self-service loan application form.
func (h *SelfServiceHandler) ApplyForLoan(c *gin.Context) {
	_, ok := h.getCurrentUser(c)
	if !ok {
		return
	}

	c.HTML(http.StatusOK, "base", PageData(c, gin.H{
		"title":         "Apply for Loan",
		"page_template": "apply_for_loan_content",
	}))
}

// ApplyForLoanSubmit handles the loan application submission.
func (h *SelfServiceHandler) ApplyForLoanSubmit(c *gin.Context) {
	user, ok := h.getCurrentUser(c)
	if !ok {
		return
	}

	var request models.SelfServiceLoanRequest
	if err := c.ShouldBind(&request); err != nil {
		c.HTML(http.StatusUnprocessableEntity, "base", PageData(c, gin.H{
			"title":         "Apply for Loan",
			"page_template": "apply_for_loan_content",
			"error":         "Please complete all required fields.",
		}))
		return
	}

	loan, err := h.loanService.ApplyForLoanForUser(user.ID, user.Email, request)
	if err != nil {
		message := "Unable to submit loan application."
		if errors.Is(err, services.ErrPersonNotLinked) {
			message = "No beneficiary profile is linked to this account."
		}
		c.HTML(http.StatusUnprocessableEntity, "base", PageData(c, gin.H{
			"title":         "Apply for Loan",
			"page_template": "apply_for_loan_content",
			"error":         message,
		}))
		return
	}

	// Audit log
	if h.auditLogService != nil {
		_ = h.auditLogService.Create(
			user.ID.String(),
			"CREATE_LOAN_APPLICATION",
			"loans",
			loan.ID.String(),
			"Self-service loan application submitted",
		)
	}

	c.Redirect(http.StatusSeeOther, "/my/loans?submitted=1")
}

// MyRepayments renders the user's repayment schedule.
func (h *SelfServiceHandler) MyRepayments(c *gin.Context) {
	user, ok := h.getCurrentUser(c)
	if !ok {
		return
	}

	repayments, err := h.loanRepaymentService.GetRepaymentsForUser(user.Email)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "base", PageData(c, gin.H{
			"title":         "My Repayments",
			"page_template": "my_repayments_content",
			"error":         "Unable to load repayments.",
		}))
		return
	}

	c.HTML(http.StatusOK, "base", PageData(c, gin.H{
		"title":         "My Repayments",
		"page_template": "my_repayments_content",
		"repayments":    repayments,
	}))
}

// MyCare renders the user's care records.
func (h *SelfServiceHandler) MyCare(c *gin.Context) {
	user, ok := h.getCurrentUser(c)
	if !ok {
		return
	}

	records, err := h.careService.GetCareForUser(user.Email)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "base", PageData(c, gin.H{
			"title":         "My Care",
			"page_template": "my_care_content",
			"error":         "Unable to load care records.",
		}))
		return
	}

	c.HTML(http.StatusOK, "base", PageData(c, gin.H{
		"title":         "My Care",
		"page_template": "my_care_content",
		"records":       records,
	}))
}

// MyNotifications renders the user's notifications list (self-service).
// The authenticated user may only ever view their own notifications —
// the user ID is derived from the session, never from the URL.
func (h *SelfServiceHandler) MyNotifications(c *gin.Context) {
	user, ok := h.getCurrentUser(c)
	if !ok {
		return
	}

	notifications, err := h.notificationService.ListForUser(user.ID)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "base", PageData(c, gin.H{
			"title":         "Notifications",
			"page_template": "notifications_content",
			"error":         "Unable to load notifications.",
		}))
		return
	}

	c.HTML(http.StatusOK, "base", PageData(c, gin.H{
		"title":         "Notifications",
		"page_template": "notifications_content",
		"notifications": notifications,
	}))
}

// MarkNotificationRead marks a notification as read for the authenticated user.
// Ownership is enforced server-side: the user ID is taken from the session, so
// a user can never mark another user's notification as read (IDOR protection).
func (h *SelfServiceHandler) MarkNotificationRead(c *gin.Context) {
	user, ok := h.getCurrentUser(c)
	if !ok {
		return
	}

	id := c.Param("id")
	if err := h.notificationService.MarkAsRead(id, user.ID); err != nil {
		if errors.Is(err, services.ErrInvalidNotification) ||
			errors.Is(err, repository.ErrNotificationNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "Notification not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Unable to mark notification as read",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Notification marked as read",
	})
}
