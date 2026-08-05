package models

type UpdateStudentRequest struct {
	PersonID      string `json:"person_id" binding:"required"`
	FullName      string `json:"full_name" binding:"required,min=3,max=150"`
	SchoolName    string `json:"school_name" binding:"required,min=2,max=150"`
	Grade         string `json:"grade" binding:"required,max=30"`
	StudentCode   string `json:"student_code" binding:"omitempty,max=50"`
	DateOfBirth   string `json:"date_of_birth"`
	Gender        string `json:"gender" binding:"omitempty,oneof=Male Female Other"`
	GuardianName  string `json:"guardian_name" binding:"omitempty,max=150"`
	GuardianPhone string `json:"guardian_phone" binding:"omitempty,max=20"`
	AcademicYear  int    `json:"academic_year" binding:"required,gte=2000,lte=2100"`
	Remarks       string `json:"remarks"`
	Status        string `json:"status" binding:"required"`
}
