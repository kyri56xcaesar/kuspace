// uploading files helpers,.. i dont like these (might refactor)
// just to update a label 


function fileUploadContainerFunctionality(dropZone, fileInput, fileBoxContainer, submitButton, fileNameDisplay) {
    let filesList = [];
  
    function handleFileSelection(selectedFiles) {
      selectedFiles.forEach((file) => {
        if (!isDuplicate(file)) {
          filesList.push(file);
          addFileBox(file);
        }
      });
  
      updateFileInput();
      updateFileNameDisplay();
      toggleSubmitButton();
    }

    function updateFileNameDisplay() {
        fileNameDisplay.textContent = 
          filesList.length > 0 
            ? `${filesList.length} file(s) selected` 
            : "No files selected";
    }
  
    function isDuplicate(file) {
      return filesList.some(
        (f) =>
          f.name === file.name &&
          f.size === file.size &&
          f.lastModified === file.lastModified
      );
    }
  
    function addFileBox(file) {
      const fileBox = document.createElement("div");
      fileBox.classList.add("file-box", getFileClass(file.name));
  
      const fileNameSpan = document.createElement("span");
      fileNameSpan.textContent = file.name;
      fileBox.appendChild(fileNameSpan);
  
      const closeButton = document.createElement("span");
      closeButton.classList.add("close-btn");
      closeButton.textContent = "✖";
      closeButton.addEventListener("click", () => {
        removeFile(file);
        fileBox.remove();
      });
  
      fileBox.appendChild(closeButton);
      fileBoxContainer.appendChild(fileBox);
    }
  
    function removeFile(file) {
      filesList = filesList.filter(
        (f) =>
          !(
            f.name === file.name &&
            f.size === file.size &&
            f.lastModified === file.lastModified
          )
      );
      updateFileInput();
      updateFileNameDisplay();
      toggleSubmitButton();
    }
  
    function updateFileInput() {
      const dataTransfer = new DataTransfer();
      filesList.forEach((file) => dataTransfer.items.add(file));
      fileInput.files = dataTransfer.files;
    }
  
    function updateFileNameDisplay() {
      fileNameDisplay.textContent =
        filesList.length === 0
          ? "No file selected"
          : `${filesList.length} file(s) selected`;
    }
  
    function toggleSubmitButton() {
      submitButton.disabled = filesList.length === 0;
    }
  
    function getFileClass(filename) {
      const ext = filename.split(".").pop().toLowerCase();
      return {
        jpg: "image",
        jpeg: "image",
        png: "image",
        gif: "image",
        pdf: "pdf",
        doc: "doc",
        docx: "doc",
        zip: "zip",
        rar: "zip",
      }[ext] || "default";
    }
  
    // Drag and drop handlers
    dropZone.addEventListener("dragover", (e) => {
      e.preventDefault();
      dropZone.classList.add("drag-over");
    });
  
    dropZone.addEventListener("dragleave", () => {
      dropZone.classList.remove("drag-over");
    });
  
    dropZone.addEventListener("drop", (e) => {
      e.preventDefault();
      dropZone.classList.remove("drag-over");
  
      const droppedFiles = Array.from(e.dataTransfer.files);
      handleFileSelection(droppedFiles);
    });
  
    fileInput.addEventListener("change", (e) => {
      const selectedFiles = Array.from(e.target.files);
      handleFileSelection(selectedFiles);
    });
  
    // Return a reset hook
    return {
      reset: () => {
        filesList = [];
        fileInput.value = "";
        updateFileInput();
        updateFileNameDisplay();
        toggleSubmitButton();

        fileBoxContainer.querySelectorAll(".file-box").forEach((box) => box.remove())
      },
    };
}
  