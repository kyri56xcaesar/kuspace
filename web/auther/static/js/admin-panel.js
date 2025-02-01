function editUser(uid, index) {
  const row = document.getElementById(`table-${index}`);
  if (!row) return;

  const cells = row.querySelectorAll('td');
  if (!cells) return;

  const originalValues = {};

  for (let i = 0; i < cells.length - 1; i++) {
    const cell = cells[i];
    const originalText = cell.textContent.trim();

    originalValues[i] = originalText;

    if (i == 0) {
      continue;
    }

    if (i == 5) {
      continue;
    }

    const input = document.createElement('input');
    input.type = 'text';
    //input.value = originalText;
    input.id = 'edit-input-'+uid+'-'+i;
    input.classList.add("table-input");
    input.placeholder = originalText;
    input.dataset.index = i;
    cell.innerHTML = '';
    cell.appendChild(input);
  }

  const actionsCell = cells[cells.length - 1];
  actionsCell.innerHTML = `
    <div id="actions-btns">
      <button 
        id="submit-btn-${index}" 
        hx-patch="/api/v1/verified/admin/userpatch"
        hx-swap="none"
        hx-trigger="click"
        hx-confirm="Are you sure you want to update user ${originalValues[0]}?"
        hx-vals="js:{...getUserPatchValues(${uid})}"

        type="button"
      >
        Submit
      </button>
      <button id="cancel-btn-${index}" onclick='cancelEdit(${index}, ${JSON.stringify(originalValues).replace(/'/g, "\\'")})'>Cancel</button>
    </div>
  `;
  htmx.process(document.getElementById(`submit-btn-${index}`));

}

function getUserPatchValues(uid) {
  let ed1 = document.getElementById("edit-input-"+uid+"-1");
  let ed2 = document.getElementById("edit-input-"+uid+"-2");
  let ed3 = document.getElementById("edit-input-"+uid+"-3");
  let ed4 = document.getElementById("edit-input-"+uid+"-4");
  let ed6 = document.getElementById("edit-input-"+uid+"-6");
  r = {
    uid: uid,
    username : ed1.value,
    password : ed2.value,
    home: ed3.value,
    shell: ed4.value,
    groups: ed6.value
  };

  return r
}

function cancelEdit(index, originalValues) {
  const row = document.getElementById(`table-${index}`);
  if (!row) return;

  const cells = row.querySelectorAll('td');
  
  // Restore original cell values
  for (let i = 0; i < cells.length - 1; i++) {
    cells[i].innerHTML = originalValues[i];
  }


  // Restore actions cell
  const actionsCell = cells[cells.length - 1];
  actionsCell.innerHTML = `
    <div id="actions-btns">
      <button id="edit-btn-${index}" onclick="editUser('${originalValues[0]}', ${index})">Edit</button>
      <button 
        id="delete-btn-${index}"
        hx-delete="/api/v1/verified/admin/userdel?uid=${originalValues[0]}"
        hx-swap="none"
        hx-trigger="click"
        hx-target="#table-${index}"
        hx-confirm="Are you sure you want to delete user ${originalValues[0]}?"
      >Delete</button>
    </div>
  `;
  htmx.process(document.getElementById(`delete-btn-${index}`));

}

function showProgressBar(container) {
  if (container){
    container.classList.remove('hidden');
  }
}

function hideProgressBar(container) {
  if (container) {
    container.classList.add('hidden');
  } 
}

var filesList = [];

document.addEventListener("DOMContentLoaded", () => {
  const sidebar = document.getElementById('sidebar');
  const toggleSidebarButton = document.getElementById('toggle-sidebar');
  const sidebarList = document.querySelectorAll('.collapsing');
  toggleSidebarButton.addEventListener('click', () => {
    sidebar.classList.toggle('collapsed');
    sidebarList.forEach((item) => {
      if (sidebar.classList.contains('collapsed')) {
        item.style.opacity = '0';
        item.style.pointerEvents = 'none'; // Prevent interaction when hidden
      } else {
        item.style.opacity = '1';
        item.style.pointerEvents = 'auto';
      }
    });
  });
  const dropZone = document.getElementById("drop-zone");
  const fileInput = document.getElementById("file");
  const fileBoxContainer = document.getElementById("file-boxes");

  // Handle dragover event (to show visual feedback)
  dropZone.addEventListener("dragover", (event) => {
    event.preventDefault(); // Prevent default behavior (like opening the file in the browser)
    dropZone.classList.add("drag-over");
  });

  // Handle dragleave event (to remove visual feedback)
  dropZone.addEventListener("dragleave", () => {
    dropZone.classList.remove("drag-over");
  });

  // Handle drop event
  dropZone.addEventListener("drop", (event) => {
    event.preventDefault(); // Prevent default behavior
    dropZone.classList.remove("drag-over");
    
    const files = Array.from(event.dataTransfer.files);
    if (files.length > 0) {
      handleFileSelectionFromDrop(files);
    }
  });
  
  function toggleSubmitButton() {
    const fileInput = document.getElementById("file");
    const submitButton = document.getElementById("upload-button");

    submitButton.disabled = fileInput.files.length === 0;
  }
  
  function handleFileSelection(event) {
    toggleSubmitButton();

    const selectedFiles = Array.from(event.target.files);
    
    selectedFiles.forEach((file) => {
      if (!filesList.some((f) => f.name === file.name)) {
        filesList.push(file);
        addFileBox(file);
      }
    });
    
     updateFileNameDisplay(filesList);
  }

  function handleFileSelectionFromDrop(files) {
    const selectedFiles = Array.from(files);
    selectedFiles.forEach((file) => {
      if (!filesList.some((f) => f.name === file.name)) {
        filesList.push(file);
        addFileBox(file);
      }
    });
    
    updateFileNameDisplay(filesList);
  }

  function addFileBox(file) {
    const fileBox = document.createElement("div");
    fileBox.classList.add("file-box");

    const extention = file.name.split(".").pop().toLowerCase();
    const fileClass = getFileClass(extention);

    fileBox.classList.add("file-box", fileClass);

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
    filesList = filesList.filter((f) => f.name !== file.name);
    updateFileNameDisplay(filesList);
  }
  // Handle file selection via the input field
  fileInput.addEventListener("change", handleFileSelection)

  // Function to handle files
  function handleFiles(files) {
    const file = files[0];
    if (file) {
      fileNameDisplay.textContent = file.name;

      // You can now process the file, e.g., upload it to a server
      console.log("File selected:", file);
    }
  }

  function getFileClass(extension) {
    switch (extension) {
    case "jpg":
    case "jpeg":
    case "png":
    case "gif":
      return "image";
    case "pdf":
      return "pdf";
    case "doc":
    case "docx":
      return "doc";
    case "zip":
    case "rar":
      return "zip";
    default:
      return "default";
    }
  }

});

function updateFileNameDisplay(filesList) {
    const fileNameLabel = document.getElementById("file-name");
    fileNameLabel.textContent = 
      filesList.length > 0 
        ? `${filesList.length} file(s) selected` 
        : "No files selected";
  }


function resetFileBoxDisplay() {
  const fileboxes = document.querySelectorAll(".file-box");
  fileboxes.forEach((file_box) => {
    file_box.remove();
  });

  updateFileNameDisplay([]);
}
 
function toggleDynamicQuota(checkbox) {
  const ranges = document.querySelectorAll('.quota-range');
  ranges.forEach(range => {
      if (checkbox.checked) {
          range.classList.add('disabled');
      } else {
          range.classList.remove('disabled');
      }
  });
}