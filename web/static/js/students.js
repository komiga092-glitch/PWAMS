"use strict";
/* =========================================================
   DOM ELEMENTS
   ========================================================= */
const studentModal = document.getElementById("student-modal");
const studentForm = document.getElementById("student-create-form");
const addStudentButton = document.getElementById("add-student-btn");
const closeButton = document.getElementById("student-modal-close");
const cancelButton = document.getElementById("student-cancel");
const personSelect = document.getElementById("student_person_id");
/* =========================================================
   OPEN MODAL
   ========================================================= */
function openStudentModal() {
    if (!studentModal) {
        console.error("Student modal not found.");
        return;
    }
    studentModal.style.display = "flex";
    document.body.style.overflow = "hidden";
    void loadPersons();
}
/* =========================================================
   CLOSE MODAL
   ========================================================= */
function closeStudentModal() {
    if (!studentModal) {
        return;
    }
    studentModal.style.display = "none";
    document.body.style.overflow = "";
    resetStudentForm();
}
/* =========================================================
   RESET FORM
   ========================================================= */
function resetStudentForm() {
    if (!studentForm) {
        return;
    }
    studentForm.reset();
    if (personSelect) {
        personSelect.innerHTML =
            '<option value="">Select Person</option>';
    }
}
/* =========================================================
   GET FORM DATA
   ========================================================= */
function getStudentFormData(form) {
    const formData = new FormData(form);
    return {
        person_id: String(formData.get("person_id") ?? "").trim(),
        full_name: String(formData.get("full_name") ?? "").trim(),
        school_name: String(formData.get("school_name") ?? "").trim(),
        grade: String(formData.get("grade") ?? "").trim(),
        student_code: String(formData.get("student_code") ?? "").trim(),
        date_of_birth: String(formData.get("date_of_birth") ?? "").trim(),
        gender: String(formData.get("gender") ?? "").trim(),
        guardian_name: String(formData.get("guardian_name") ?? "").trim(),
        guardian_phone: String(formData.get("guardian_phone") ?? "").trim(),
        academic_year: Number(formData.get("academic_year") ?? 0),
        remarks: String(formData.get("remarks") ?? "").trim()
    };
}
/* =========================================================
   VALIDATE FORM
   ========================================================= */
function validateStudentData(data) {
    if (!data.person_id) {
        return "Please select a person.";
    }
    if (!data.full_name) {
        return "Full name is required.";
    }
    if (!data.school_name) {
        return "School / Institution is required.";
    }
    if (!data.grade) {
        return "Grade is required.";
    }
    if (!data.academic_year) {
        return "Academic year is required.";
    }
    if (data.academic_year < 2000 ||
        data.academic_year > 2100) {
        return "Please enter a valid academic year.";
    }
    return null;
}
/* =========================================================
   CREATE STUDENT
   ========================================================= */
async function createStudent(data) {
    const response = await fetch("/students", {
        method: "POST",
        headers: {
            "Content-Type": "application/json",
            "Accept": "application/json"
        },
        body: JSON.stringify(data)
    });
    let result = {};
    try {
        result =
            await response.json();
    }
    catch {
        /*
         * Backend may return empty response.
         * We handle that below.
         */
    }
    if (!response.ok) {
        const message = result.message ||
            result.error ||
            "Failed to create student.";
        throw new Error(message);
    }
    if (result.success === false) {
        throw new Error(result.message ||
            "Failed to create student.");
    }
}
/* =========================================================
   LOAD PERSONS
   ========================================================= */
async function loadPersons() {
    if (!personSelect) {
        console.error("Student person select element not found.");
        return;
    }
    /*
     * Loading state
     */
    personSelect.innerHTML =
        '<option value="">Loading persons...</option>';
    personSelect.disabled = true;
    try {
        const response = await fetch("/persons", {
            method: "GET",
            headers: {
                "Accept": "application/json"
            }
        });
        if (!response.ok) {
            throw new Error(`Failed to load persons (${response.status})`);
        }
        const result = await response.json();
        /*
         * Clear existing options
         */
        personSelect.innerHTML =
            '<option value="">Select Person</option>';
        /*
         * Check response
         */
        if (!result.success ||
            !Array.isArray(result.data)) {
            throw new Error(result.message ||
                "Invalid persons response.");
        }
        /*
         * No persons available
         */
        if (result.data.length === 0) {
            personSelect.innerHTML =
                '<option value="">No persons available</option>';
            return;
        }
        /*
         * Add persons
         */
        for (const person of result.data) {
            const option = document.createElement("option");
            option.value = person.id;
            option.textContent =
                person.full_name;
            /*
             * Store phone if needed later
             */
            option.dataset.phone =
                person.phone;
            /*
             * Store status if needed later
             */
            option.dataset.status =
                person.status;
            personSelect.appendChild(option);
        }
    }
    catch (error) {
        console.error("Unable to load persons:", error);
        personSelect.innerHTML =
            '<option value="">Failed to load persons</option>';
    }
    finally {
        personSelect.disabled = false;
    }
}
/* =========================================================
   ADD STUDENT BUTTON
   ========================================================= */
addStudentButton?.addEventListener("click", () => {
    openStudentModal();
});
/* =========================================================
   CLOSE BUTTON
   ========================================================= */
closeButton?.addEventListener("click", () => {
    closeStudentModal();
});
/* =========================================================
   CANCEL BUTTON
   ========================================================= */
cancelButton?.addEventListener("click", () => {
    closeStudentModal();
});
/* =========================================================
   CLICK OUTSIDE MODAL
   ========================================================= */
studentModal?.addEventListener("click", (event) => {
    if (event.target === studentModal) {
        closeStudentModal();
    }
});
/* =========================================================
   ESC KEY
   ========================================================= */
document.addEventListener("keydown", (event) => {
    if (event.key === "Escape" &&
        studentModal?.style.display === "flex") {
        closeStudentModal();
    }
});
/* =========================================================
   FORM SUBMIT
   ========================================================= */
studentForm?.addEventListener("submit", async (event) => {
    event.preventDefault();
    const submitButton = studentForm.querySelector('button[type="submit"]');
    try {
        /*
         * Get form data
         */
        const data = getStudentFormData(studentForm);
        /*
         * Validate
         */
        const validationError = validateStudentData(data);
        if (validationError) {
            alert(validationError);
            return;
        }
        /*
         * Disable submit button
         */
        if (submitButton) {
            submitButton.disabled = true;
            submitButton.textContent =
                "Saving...";
        }
        /*
         * Create student
         */
        await createStudent(data);
        /*
         * Success
         */
        alert("Student created successfully");
        /*
         * Close modal
         */
        closeStudentModal();
        /*
         * Reload student list
         *
         * This is important because
         * /students/page will now
         * fetch the newly created student.
         */
        window.location.reload();
    }
    catch (error) {
        console.error("Student creation failed:", error);
        const message = error instanceof Error
            ? error.message
            : "Failed to create student.";
        alert(message);
    }
    finally {
        /*
         * Enable submit button
         */
        if (submitButton) {
            submitButton.disabled = false;
            submitButton.textContent =
                "Save Student";
        }
    }
});
//# sourceMappingURL=students.js.map