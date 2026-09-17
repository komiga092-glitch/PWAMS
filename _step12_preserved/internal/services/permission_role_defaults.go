package services

// defaultRolePermissions maps each role name to its default set of permission
// names. These are seeded on startup and form the baseline authorization
// matrix for every role.
func defaultRolePermissions() map[string][]string {
	allPerms := permissionNames(allPermissions())

	return map[string][]string{
		"Super Admin": allPerms, // Full access — every permission.

		"Admin": {
			// Admin account management (multi-Admin): the Admin role holds the
			// admin.* namespace so an Admin can create and manage other Admin
			// accounts. Deletion is a two-step workflow — admin.delete.request
			// raises a deletion request, admin.delete.approve executes it and
			// admin.delete.reject turns it down; the service layer enforces a
			// four-eyes rule (the approver of a request can never be its
			// requester). Super Admin accounts remain exclusively managed by
			// the Super Admin role (super_admin.* is never granted here).
			//
			// IMPORTANT SEPARATION OF CAPABILITIES:
			//   A) managing Admin *user accounts*        -> admin.* (below)
			//   B) modifying the Admin *role definition*  -> permission
			//      designer on the Admin/Super Admin role sets, reserved for
			//      the Super Admin role (services.CanManageRolePermissions +
			//      handlers.designerRoles). Holding admin.* therefore never
			//      lets an Admin redesign its own role or its permissions.
			"admin.view", "admin.create", "admin.edit",
			"admin.activate", "admin.deactivate",
			"admin.delete.request", "admin.delete.approve", "admin.delete.reject",
			// User / staff / volunteer / partner account management.
			// users.delete implements the final delete policy: the Admin
			// deletes normal user accounts directly. Hierarchy protection is
			// enforced server-side on every route — Super Admin targets are
			// rejected for any non-Super-Admin actor, and Admin targets are
			// only removable through the supervised two-person workflow.
			"users.view", "users.create", "users.edit", "users.delete",
			"users.change_role", "users.activate", "users.enable", "users.disable", "users.reset_password",
			"partner.view", "partner.create", "partner.edit", "partner.delete",
			"partner.activate", "partner.deactivate",
			"staff.view", "staff.create", "staff.edit", "staff.delete",
			"volunteer.view", "volunteer.create", "volunteer.edit",
			// Donor & beneficiary
			"donor.view", "donor.create", "donor.edit", "donor.delete",
			"beneficiary.view", "beneficiary.create", "beneficiary.edit", "beneficiary.delete",
			"person.view", "person.create", "person.edit", "person.delete",
			// Donations
			"donation.view", "donation.create", "donation.edit", "donation.delete",
			// Aid & care
			"aid.view", "aid.create", "aid.edit", "aid.approve",
			"care.view", "care.create", "care.edit", "care.delete",
			// Loans & repayments
			"loan.view", "loan.create", "loan.edit", "loan.approve",
			"repayment.view", "repayment.create", "repayment.edit",
			// Student
			"student.view", "student.create", "student.edit", "student.delete",
			// Files, messages
			"file.view", "file.upload", "file.delete",
			"message.view", "message.send", "message.delete",
			// Reports & audit
			"reports.view", "reports.export", "audit_logs.view", "audit_logs.export",
			// Revenue & notifications
			"revenue.view", "revenue.create",
			"notification.view",
			// Permissions & account management
			"permission_management.view", "permission_management.edit",
			"account_management.view",
		},

		// Partner = NGO Owner / NGO Management. Full NGO operational access
		// (staff, volunteers, donors, beneficiaries, students, donations, aid,
		// loans, repayments, care, files, reports, notifications) but NEVER
		// Admin Management (admin.*), Super Admin accounts, system
		// configuration, protected RBAC, or permission management. Account
		// targets are additionally restricted by the role hierarchy
		// (services.CanAssignRole / services.CanManageAccountRole).
		"Partner": {
			// Operational account management (staff / volunteer / donor /
			// beneficiary / student accounts; target role restricted
			// server-side by the role hierarchy).
			"users.view", "users.create", "users.edit",
			"users.change_role", "users.activate", "users.enable", "users.disable", "users.reset_password",
			// Staff & volunteers
			"staff.view", "staff.create", "staff.edit", "staff.delete",
			"volunteer.view", "volunteer.create", "volunteer.edit",
			// Donor & beneficiary
			"donor.view", "donor.create", "donor.edit", "donor.delete",
			"beneficiary.view", "beneficiary.create", "beneficiary.edit", "beneficiary.delete",
			"person.view", "person.create", "person.edit", "person.delete",
			// Donations
			"donation.view", "donation.create", "donation.edit", "donation.delete",
			// Aid & care
			"aid.view", "aid.create", "aid.edit", "aid.approve",
			"care.view", "care.create", "care.edit", "care.delete",
			// Loans & repayments
			"loan.view", "loan.create", "loan.edit", "loan.approve",
			"repayment.view", "repayment.create", "repayment.edit",
			// Student
			"student.view", "student.create", "student.edit", "student.delete",
			// Files, messages
			"file.view", "file.upload", "file.delete",
			"message.view", "message.send", "message.delete",
			// Reports & notifications
			"reports.view",
			"notification.view",
			// Account management console
			"account_management.view",
		},

		"Staff": {
			// Can register Donor / Beneficiary accounts (target role is
			// restricted server-side by canCreateRole).
			"users.create",
			// Donor & beneficiary
			"donor.view", "donor.create", "donor.edit",
			"beneficiary.view", "beneficiary.create", "beneficiary.edit",
			"person.view", "person.create", "person.edit",
			// Donations
			"donation.view", "donation.create", "donation.edit",
			// Aid & care
			"aid.view", "aid.create", "aid.edit",
			"care.view", "care.create", "care.edit",
			// Loans & repayments
			"loan.view", "loan.create", "loan.edit",
			"repayment.view", "repayment.create", "repayment.edit",
			// Student
			"student.view", "student.create", "student.edit",
			// Files, messages
			"file.view", "file.upload",
			"message.view", "message.send",
			// Reports & audit
			"reports.view", "audit_logs.view",
			// Notifications
			"notification.view",
		},

		"Volunteer": {
			// Can register Donor / Beneficiary accounts (target role is
			// restricted server-side by canCreateRole).
			"users.create",
			// Donor & beneficiary
			"donor.view", "donor.create", "donor.edit",
			"beneficiary.view", "beneficiary.create", "beneficiary.edit",
			"person.view",
			// Donations
			"donation.view", "donation.create",
			// Aid & care
			"aid.view", "aid.create",
			"care.view", "care.create",
			// Loans & repayments
			"loan.view", "loan.create",
			"repayment.view", "repayment.create",
			// Files, messages
			"file.view",
			"message.view",
			// Notifications
			"notification.view",
		},

		"Donor": {
			"donation.view",
			"message.view",
			"notification.view",
		},

		"Beneficiary": {
			"aid.view_own",
			"aid.request",
			"loan.view_own",
			"loan.apply",
			"repayment.view_own",
			"care.view_own",
			"message.view",
			"notification.view_own",
		},

		"Student": {
			"aid.view_own",
			"aid.request",
			"loan.view_own",
			"loan.apply",
			"repayment.view_own",
			"message.view",
			"notification.view_own",
		},
	}
}
