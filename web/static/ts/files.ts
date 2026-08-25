export {};

interface FileRecord {
  id: string;
  original_name: string;
  content_type: string;
  size: number;
  created_at: string;
}

interface FileListResponse {
  success: boolean;
  data?: FileRecord[];
  pagination?: {
    page: number;
    page_size: number;
    total_items: number;
    total_pages: number;
  };
  message?: string;
}

interface FileResponse {
  success: boolean;
  file?: FileRecord;
  message?: string;
}

const fileTable = document.getElementById("file-table") as HTMLTableElement;
const fileTableBody = document.getElementById(
  "file-table-body",
) as HTMLTableSectionElement;
const fileLoading = document.getElementById("file-loading") as HTMLDivElement;
const fileEmpty = document.getElementById("file-empty") as HTMLDivElement;
const fileError = document.getElementById("file-error") as HTMLDivElement;
const filePagination = document.getElementById(
  "file-pagination",
) as HTMLDivElement;
const fileCount = document.getElementById("file-count") as HTMLSpanElement;
const filePrevious = document.getElementById(
  "file-previous",
) as HTMLButtonElement;
const fileNext = document.getElementById("file-next") as HTMLButtonElement;
const filePageInfo = document.getElementById(
  "file-page-info",
) as HTMLSpanElement;
const fileModal = document.getElementById(
  "file-upload-modal",
) as HTMLDivElement;
const fileForm = document.getElementById("file-upload-form") as HTMLFormElement;
const fileInput = document.getElementById("file-input") as HTMLInputElement;
const fileSelectionInfo = document.getElementById(
  "file-selection-info",
) as HTMLElement;
const fileFeedback = document.getElementById(
  "file-upload-feedback",
) as HTMLDivElement;
const fileSubmit = document.getElementById(
  "file-upload-submit",
) as HTMLButtonElement;
let currentPage = 1;
let totalPages = 1;

async function request<T>(url: string, options?: RequestInit): Promise<T> {
  const response = await fetch(url, {
    credentials: "same-origin",
    headers: { Accept: "application/json", ...options?.headers },
    ...options,
  });
  const result = (await response.json().catch(() => ({}))) as T & {
    message?: string;
  };
  if (!response.ok)
    throw new Error(result.message || `Request failed (${response.status})`);
  return result;
}

function formatSize(size: number): string {
  if (size < 1024) return `${size} B`;
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`;
  return `${(size / (1024 * 1024)).toFixed(2)} MB`;
}

function formatDate(value: string): string {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
}

function showAlert(message: string, success = false): void {
  const alert = document.getElementById("file-alert") as HTMLDivElement;
  alert.className = `alert ${success ? "alert-success" : "alert-danger"}`;
  alert.textContent = message;
  alert.hidden = false;
}

function button(label: string, className: string): HTMLButtonElement {
  const result = document.createElement("button");
  result.type = "button";
  result.className = `btn btn-small ${className}`;
  result.textContent = label;
  return result;
}

function renderFiles(files: FileRecord[]): void {
  fileTableBody.replaceChildren();
  for (const file of files) {
    const row = document.createElement("tr");
    const name = document.createElement("td");
    name.textContent = file.original_name;
    const type = document.createElement("td");
    type.textContent = file.content_type;
    const size = document.createElement("td");
    size.textContent = formatSize(file.size);
    const date = document.createElement("td");
    date.textContent = formatDate(file.created_at);
    const actions = document.createElement("td");
    actions.className = "table-actions";
    const download = button("Download", "btn-secondary");
    download.addEventListener("click", () => {
      window.location.href = `/files/${encodeURIComponent(file.id)}`;
    });
    const remove = button("Delete", "btn-danger");
    remove.addEventListener("click", () => void deleteFile(file.id, remove));
    actions.append(download, remove);
    row.append(name, type, size, date, actions);
    fileTableBody.appendChild(row);
  }
}

async function loadFiles(page = 1): Promise<void> {
  currentPage = page;
  fileLoading.hidden = false;
  fileEmpty.hidden = true;
  fileError.hidden = true;
  fileTable.hidden = true;
  filePagination.hidden = true;
  try {
    const result = await request<FileListResponse>(
      `/files?page=${page}&page_size=20`,
    );
    const files = result.data || [];
    const pagination = result.pagination;
    totalPages = pagination?.total_pages || 1;
    fileCount.textContent = `${pagination?.total_items || files.length} Records`;
    fileLoading.hidden = true;
    if (!files.length) {
      fileEmpty.hidden = false;
      return;
    }
    renderFiles(files);
    fileTable.hidden = false;
    if (totalPages > 1) {
      filePageInfo.textContent = `Page ${pagination?.page || page} of ${totalPages}`;
      filePrevious.disabled = page <= 1;
      fileNext.disabled = page >= totalPages;
      filePagination.hidden = false;
    }
  } catch (error) {
    fileLoading.hidden = true;
    fileError.textContent =
      error instanceof Error ? error.message : "Unable to load files.";
    fileError.hidden = false;
  }
}

function openUpload(): void {
  fileModal.classList.remove("hidden");
  document.body.style.overflow = "hidden";
  fileFeedback.hidden = true;
  fileInput.focus();
}

function closeUpload(): void {
  fileModal.classList.add("hidden");
  document.body.style.overflow = "";
  fileForm.reset();
  fileSelectionInfo.textContent = "";
  fileFeedback.hidden = true;
  fileSubmit.disabled = false;
  fileSubmit.textContent = "Upload File";
}

async function uploadFile(event: SubmitEvent): Promise<void> {
  event.preventDefault();
  const file = fileInput.files?.[0];
  if (!file) {
    fileFeedback.textContent = "Please select a file.";
    fileFeedback.className = "form-message error";
    fileFeedback.hidden = false;
    return;
  }
  fileSubmit.disabled = true;
  fileSubmit.textContent = "Uploading...";
  fileFeedback.hidden = true;
  try {
    const formData = new FormData();
    formData.append("file", file, file.name);
    await request<FileResponse>("/files/upload", {
      method: "POST",
      body: formData,
    });
    closeUpload();
    showAlert("File uploaded successfully.", true);
    await loadFiles(1);
  } catch (error) {
    fileFeedback.textContent =
      error instanceof Error ? error.message : "Unable to upload file.";
    fileFeedback.className = "form-message error";
    fileFeedback.hidden = false;
    fileSubmit.disabled = false;
    fileSubmit.textContent = "Upload File";
  }
}

async function deleteFile(
  id: string,
  deleteButton: HTMLButtonElement,
): Promise<void> {
  if (!window.confirm("Delete this file?")) return;
  deleteButton.disabled = true;
  try {
    await request<FileResponse>(`/files/${encodeURIComponent(id)}`, {
      method: "DELETE",
    });
    showAlert("File deleted successfully.", true);
    await loadFiles(currentPage);
  } catch (error) {
    deleteButton.disabled = false;
    showAlert(
      error instanceof Error ? error.message : "Unable to delete file.",
    );
  }
}

fileInput.addEventListener("change", () => {
  const file = fileInput.files?.[0];
  fileSelectionInfo.textContent = file
    ? `${file.name} (${formatSize(file.size)})`
    : "";
});
document
  .getElementById("open-file-upload")
  ?.addEventListener("click", openUpload);
document
  .getElementById("file-upload-close")
  ?.addEventListener("click", closeUpload);
document
  .getElementById("file-upload-cancel")
  ?.addEventListener("click", closeUpload);
fileModal
  .querySelector(".modal-backdrop")
  ?.addEventListener("click", closeUpload);
fileForm.addEventListener("submit", (event) => void uploadFile(event));
filePrevious.addEventListener("click", () => void loadFiles(currentPage - 1));
fileNext.addEventListener("click", () => void loadFiles(currentPage + 1));
document.addEventListener("keydown", (event) => {
  if (event.key === "Escape" && !fileModal.classList.contains("hidden"))
    closeUpload();
});

void loadFiles();
