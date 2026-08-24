interface UpdateStudentRequest {
  person_id: string;
  full_name: string;
  school_name: string;
  grade: string;
  student_code: string;
  date_of_birth: string;
  gender: string;
  guardian_name: string;
  guardian_phone: string;
  academic_year: number;
  remarks: string;
  status: string;
}

interface UpdateResponse {
  success: boolean;
  message?: string;
}

const editForm = document.getElementById(
  "student-edit-form",
) as HTMLFormElement | null;

async function updateStudent(
  studentID: string,
  data: UpdateStudentRequest,
): Promise<void> {
  if (!navigator.onLine) {
    const { savePendingMutation, offlineSuccessMessage } = await import(
      String("/static/js/offline/mutations.js")
    );
    await savePendingMutation(
      "student",
      "UPDATE",
      data as unknown as Record<string, unknown>,
      studentID,
    );
    alert(offlineSuccessMessage("student", "UPDATE"));
    return;
  }

  const response = await fetch(`/students/${studentID}`, {
    method: "PUT",

    headers: {
      "Content-Type": "application/json",
      Accept: "application/json",
    },

    body: JSON.stringify(data),
  });

  let result: UpdateResponse = {
    success: false,
  };

  try {
    result = (await response.json()) as UpdateResponse;
  } catch {
    // Ignore invalid JSON
  }

  if (!response.ok || !result.success) {
    throw new Error(result.message || "Failed to update student.");
  }
}

editForm?.addEventListener("submit", async (event: SubmitEvent) => {
  event.preventDefault();

  const studentID = editForm.dataset.studentId;

  if (!studentID) {
    alert("Student ID is missing.");

    return;
  }

  const formData = new FormData(editForm);

  const data: UpdateStudentRequest = {
    person_id: String(formData.get("person_id") ?? ""),

    full_name: String(formData.get("full_name") ?? "").trim(),

    school_name: String(formData.get("school_name") ?? "").trim(),

    grade: String(formData.get("grade") ?? "").trim(),

    student_code: String(formData.get("student_code") ?? "").trim(),

    date_of_birth: String(formData.get("date_of_birth") ?? ""),

    gender: String(formData.get("gender") ?? ""),

    guardian_name: String(formData.get("guardian_name") ?? "").trim(),

    guardian_phone: String(formData.get("guardian_phone") ?? "").trim(),

    academic_year: Number(formData.get("academic_year") ?? 0),

    remarks: String(formData.get("remarks") ?? "").trim(),

    status: String(formData.get("status") ?? "Active"),
  };

  /*
   * person_id is required by UpdateStudentRequest.
   *
   * Since this page already knows the student,
   * use the person ID rendered by the backend.
   */
  if (!data.person_id) {
    const personID = editForm.dataset.personId;

    if (personID) {
      data.person_id = personID;
    }
  }

  const submitButton = editForm.querySelector(
    'button[type="submit"]',
  ) as HTMLButtonElement | null;

  try {
    if (submitButton) {
      submitButton.disabled = true;

      submitButton.textContent = "Updating...";
    }

    await updateStudent(studentID, data);

    alert("Student updated successfully.");

    if (!navigator.onLine) return;

    window.location.href = `/students/${studentID}/view`;
  } catch (error) {
    console.error("Student update failed:", error);

    alert(error instanceof Error ? error.message : "Failed to update student.");
  } finally {
    if (submitButton) {
      submitButton.disabled = false;

      submitButton.textContent = "Update Student";
    }
  }
});
