package services

import "github.com/komiga092-glitch/pwams/internal/models"

// AllPermissions returns the complete canonical list of permissions.
func AllPermissions() []models.Permission {
	return allPermissions()
}

func allPermissions() []models.Permission {
	return []models.Permission{
		// Users / system accounts
		{Name: "users.view", Description: "View user accounts", Category: "users"},
		{Name: "users.create", Description: "Create user accounts", Category: "users"},
		{Name: "users.edit", Description: "Edit user accounts", Category: "users"},
		{Name: "users.delete", Description: "Delete user accounts", Category: "users"},
		{Name: "users.change_role", Description: "Change a user's role", Category: "users"},
		{Name: "users.activate", Description: "Activate a user account", Category: "users"},
		{Name: "users.disable", Description: "Disable a user account", Category: "users"},
		{Name: "users.enable", Description: "Re-enable a disabled user account", Category: "users"},
		{Name: "users.reset_password", Description: "Reset a user's password", Category: "users"},
		// Super Admin accounts
		{Name: "super_admin.view", Description: "View Super Admin accounts", Category: "super_admin"},
		{Name: "super_admin.create", Description: "Create Super Admin accounts", Category: "super_admin"},
		{Name: "super_admin.edit", Description: "Edit Super Admin accounts", Category: "super_admin"},
		{Name: "super_admin.deactivate", Description: "Request deactivation of a Super Admin", Category: "super_admin"},
		// Admin accounts — managed by Super Admin and Admin (multi-Admin);
		// admin.* is granted to the Super Admin and Admin roles. Deleting an
		// Admin account is split into a request/approve/reject workflow so a
		// single actor can never unilaterally remove an Admin through the
		// workflow (the approver must differ from the requester).
		{Name: "admin.view", Description: "View Admin accounts", Category: "admin"},
		{Name: "admin.create", Description: "Create Admin accounts", Category: "admin"},
		{Name: "admin.edit", Description: "Edit Admin accounts", Category: "admin"},
		{Name: "admin.activate", Description: "Activate an Admin account", Category: "admin"},
		{Name: "admin.deactivate", Description: "Deactivate an Admin account", Category: "admin"},
		{Name: "admin.delete.request", Description: "Request the deletion of an Admin account", Category: "admin"},
		{Name: "admin.delete.approve", Description: "Approve an Admin deletion request (executes the deletion) or delete an Admin account directly", Category: "admin"},
		{Name: "admin.delete.reject", Description: "Reject an Admin deletion request", Category: "admin"},
		// Partner accounts — managed by Admin, never by Partner itself
		{Name: "partner.view", Description: "View Partner accounts", Category: "partner"},
		{Name: "partner.create", Description: "Create Partner accounts", Category: "partner"},
		{Name: "partner.edit", Description: "Edit Partner accounts", Category: "partner"},
		{Name: "partner.delete", Description: "Delete Partner accounts", Category: "partner"},
		{Name: "partner.activate", Description: "Activate a Partner account", Category: "partner"},
		{Name: "partner.deactivate", Description: "Deactivate a Partner account", Category: "partner"},
		// Staff
		{Name: "staff.view", Description: "View staff members", Category: "staff"},
		{Name: "staff.create", Description: "Create staff members", Category: "staff"},
		{Name: "staff.edit", Description: "Edit staff members", Category: "staff"},
		{Name: "staff.delete", Description: "Delete staff members", Category: "staff"},
		// Volunteers
		{Name: "volunteer.view", Description: "View volunteers", Category: "volunteer"},
		{Name: "volunteer.create", Description: "Create volunteers", Category: "volunteer"},
		{Name: "volunteer.edit", Description: "Edit volunteers", Category: "volunteer"},
		// Donors
		{Name: "donor.view", Description: "View donors", Category: "donor"},
		{Name: "donor.create", Description: "Create donors", Category: "donor"},
		{Name: "donor.edit", Description: "Edit donors", Category: "donor"},
		{Name: "donor.delete", Description: "Delete donors", Category: "donor"},
		// Beneficiaries
		{Name: "beneficiary.view", Description: "View beneficiaries", Category: "beneficiary"},
		{Name: "beneficiary.create", Description: "Create beneficiaries", Category: "beneficiary"},
		{Name: "beneficiary.edit", Description: "Edit beneficiaries", Category: "beneficiary"},
		{Name: "beneficiary.delete", Description: "Delete beneficiaries", Category: "beneficiary"},
		// Person entity
		{Name: "person.view", Description: "View persons", Category: "person"},
		{Name: "person.create", Description: "Create persons", Category: "person"},
		{Name: "person.edit", Description: "Edit persons", Category: "person"},
		{Name: "person.delete", Description: "Delete persons", Category: "person"},
		// Students
		{Name: "student.view", Description: "View students", Category: "student"},
		{Name: "student.create", Description: "Create students", Category: "student"},
		{Name: "student.edit", Description: "Edit students", Category: "student"},
		{Name: "student.delete", Description: "Delete students", Category: "student"},
		// Donations
		{Name: "donation.view", Description: "View donations", Category: "donation"},
		{Name: "donation.create", Description: "Record donations", Category: "donation"},
		{Name: "donation.edit", Description: "Edit donations", Category: "donation"},
		{Name: "donation.delete", Description: "Delete donations", Category: "donation"},
		{Name: "donation.approve", Description: "Approve donations", Category: "donation"},
		// Aid & care
		{Name: "aid.view", Description: "View aid requests", Category: "aid"},
		{Name: "aid.create", Description: "Create aid requests", Category: "aid"},
		{Name: "aid.edit", Description: "Edit aid requests", Category: "aid"},
		{Name: "aid.delete", Description: "Delete aid requests", Category: "aid"},
		{Name: "aid.approve", Description: "Approve or reject aid requests", Category: "aid"},
		{Name: "aid.view_own", Description: "View own aid requests", Category: "aid"},
		{Name: "aid.request", Description: "Submit own aid request", Category: "aid"},
		{Name: "care.view", Description: "View care records", Category: "care"},
		{Name: "care.create", Description: "Create care records", Category: "care"},
		{Name: "care.edit", Description: "Edit care records", Category: "care"},
		{Name: "care.delete", Description: "Delete care records", Category: "care"},
		{Name: "care.view_own", Description: "View own care records", Category: "care"},
		// Loans & repayments
		{Name: "loan.view", Description: "View loans", Category: "loan"},
		{Name: "loan.create", Description: "Create loans", Category: "loan"},
		{Name: "loan.edit", Description: "Edit loans", Category: "loan"},
		{Name: "loan.delete", Description: "Delete loans", Category: "loan"},
		{Name: "loan.approve", Description: "Approve or reject loans", Category: "loan"},
		{Name: "loan.view_own", Description: "View own loans", Category: "loan"},
		{Name: "loan.apply", Description: "Apply for own loan", Category: "loan"},
		{Name: "repayment.view", Description: "View loan repayments", Category: "repayment"},
		{Name: "repayment.create", Description: "Record repayments", Category: "repayment"},
		{Name: "repayment.edit", Description: "Edit repayments", Category: "repayment"},
		{Name: "repayment.view_own", Description: "View own repayments", Category: "repayment"},
		// Revenue
		{Name: "revenue.view", Description: "View revenue records", Category: "revenue"},
		{Name: "revenue.create", Description: "Create revenue records", Category: "revenue"},
		{Name: "revenue.edit", Description: "Edit revenue records", Category: "revenue"},
		{Name: "revenue.delete", Description: "Delete revenue records", Category: "revenue"},
		// Files, messages & notifications
		{Name: "file.view", Description: "View uploaded files", Category: "file"},
		{Name: "file.upload", Description: "Upload files", Category: "file"},
		{Name: "file.delete", Description: "Delete files", Category: "file"},
		{Name: "message.view", Description: "View messages", Category: "message"},
		{Name: "message.send", Description: "Send messages", Category: "message"},
		{Name: "message.delete", Description: "Delete messages", Category: "message"},
		{Name: "notification.view", Description: "View all notifications", Category: "notification"},
		{Name: "notification.view_own", Description: "View own notifications", Category: "notification"},
		{Name: "notification.create", Description: "Create notifications", Category: "notification"},
		// Reports & audit
		{Name: "reports.view", Description: "View reports and dashboards", Category: "reports"},
		{Name: "reports.export", Description: "Export report data as CSV", Category: "reports"},
		{Name: "audit_logs.view", Description: "View audit logs", Category: "audit"},
		{Name: "audit_logs.export", Description: "Export audit logs", Category: "audit"},
		// Permission and account management
		{Name: "permission_management.view", Description: "View role and user permissions", Category: "permissions"},
		{Name: "permission_management.edit", Description: "Modify role and user permissions", Category: "permissions"},
		{Name: "account_management.view", Description: "Access the account management console", Category: "account"},
		// System administration — protected (Super Admin only). Foundation
		// for the upcoming system-settings module; no routes enforce these
		// permissions yet.
		{Name: "system.settings.view", Description: "View system configuration", Category: "system"},
		{Name: "system.settings.edit", Description: "Modify system configuration", Category: "system"},
		{Name: "system.maintenance", Description: "Run system maintenance operations", Category: "system"},
		// System error/alerts monitoring — protected (Super Admin by
		// default). system.alerts.view reveals the alert inbox and its
		// critical technical details; system.alerts.manage allows resolving
		// (acknowledging) alerts. Both are bound to the protected system.*
		// namespace so the permission designer can never grant them to an
		// operational role.
		{Name: "system.alerts.view", Description: "View system alerts and technical details", Category: "system"},
		{Name: "system.alerts.manage", Description: "Resolve and acknowledge system alerts", Category: "system"},
		// Protected RBAC layer — protected (Super Admin only). Guards
		// changes to the role hierarchy and protected permission namespaces.
		{Name: "protected_rbac.manage", Description: "Manage the protected RBAC layer (role hierarchy and protected permissions)", Category: "protected_rbac"},
		// Future feature namespaces (foundation only — defined so the
		// catalog is stable before the modules are implemented; the
		// permissions are not assigned to any role by default).
		{Name: "help_request.view", Description: "View help requests", Category: "help_request"},
		{Name: "help_request.create", Description: "Create help requests", Category: "help_request"},
		{Name: "help_request.edit", Description: "Edit help requests", Category: "help_request"},
		{Name: "help_request.delete", Description: "Delete help requests", Category: "help_request"},
		{Name: "help_opportunity.view", Description: "View help opportunities", Category: "help_opportunity"},
		{Name: "help_opportunity.create", Description: "Create help opportunities", Category: "help_opportunity"},
		{Name: "help_opportunity.edit", Description: "Edit help opportunities", Category: "help_opportunity"},
		{Name: "help_opportunity.delete", Description: "Delete help opportunities", Category: "help_opportunity"},
		{Name: "feedback.view", Description: "View feedback", Category: "feedback"},
		{Name: "feedback.create", Description: "Submit feedback", Category: "feedback"},
		{Name: "feedback.moderate", Description: "Moderate feedback", Category: "feedback"},
		{Name: "feedback.delete", Description: "Delete feedback", Category: "feedback"},
		{Name: "recurring_donation.view", Description: "View recurring donations", Category: "recurring_donation"},
		{Name: "recurring_donation.create", Description: "Create recurring donations", Category: "recurring_donation"},
		{Name: "recurring_donation.edit", Description: "Edit recurring donations", Category: "recurring_donation"},
		{Name: "recurring_donation.cancel", Description: "Cancel recurring donations", Category: "recurring_donation"},
		{Name: "report.export", Description: "Export reports", Category: "reports"},
		{Name: "report.pdf", Description: "Generate PDF reports", Category: "reports"},
	}
}

func permissionNames(perms []models.Permission) []string {
	names := make([]string, 0, len(perms))
	for _, p := range perms {
		names = append(names, p.Name)
	}
	return names
}
