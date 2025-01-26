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
  const fileNameDisplay = document.getElementById("file-name");

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

    const files = event.dataTransfer.files;
    if (files.length > 0) {
      handleFiles(files);
    }
  });

  // Handle file selection via the input field
  fileInput.addEventListener("change", (event) => {
    handleFiles(event.target.files);
  });

  // Function to handle files
  function handleFiles(files) {
    const file = files[0];
    if (file) {
      fileNameDisplay.textContent = file.name;

      // You can now process the file, e.g., upload it to a server
      console.log("File selected:", file);
    }
  }
});
