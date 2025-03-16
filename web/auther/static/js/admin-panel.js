/* code specific that only admin panel should use. */

/**************************************************************************/
// global variables/constants
/**************************************************************************/

// file upload list (for resources)
var filesList = [];

// what we need to prepare jobs
const modeMap = {
  "js": "javascript",
  "go": "go",
  "py": "python",
  "java": "text/x-java",
  "c": "text/x-csrc",
  "javascript":"javascript",
  "python":"python",

};

const extMap = {
  "js":"javascript",
  "py":"python",
  "go":"go",
  "c":"c",
  "java":"java",
};

const defaultMap = {
  "javascript": 
`
function run(data) {
  return data
}





`,
  "python":
`
def run(data):
    return data






`,
  "go":
`
func run(data str) str {
  return data
}





`,
  "c":
`
void run(char *buffer) {

}





`,
  "java":
`
public static String run(String data) {
  return data;
}





`,
}

/************************************************************************** */
// global functions/utilities
/**************************************************************************/

// user entries control 
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

// uploading files helpers,.. i dont like these (might refactor)
// just to update a label 
function updateFileNameDisplay(filesList) {
    const fileNameLabel = document.getElementById("file-name");
    fileNameLabel.textContent = 
      filesList.length > 0 
        ? `${filesList.length} file(s) selected` 
        : "No files selected";
}
// removes what matches the argument also
function resetFileBoxDisplay(classMatch) {
  const fileboxes = document.querySelectorAll(classMatch);
  fileboxes.forEach((file_box) => {
    file_box.remove();
  });

  updateFileNameDisplay([]);
}

// enable dynamic quota checkbox functionality
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

// "resource" file editting, permissions..
function updatePermissionString() {
  // We assume 9 bits: owner r/w/x, group r/w/x, other r/w/x
  // Grab the checkboxes in order
  const ownerR = document.querySelector('input[name="owner-r"]').checked ? 'r' : '-';
  const ownerW = document.querySelector('input[name="owner-w"]').checked ? 'w' : '-';
  const ownerX = document.querySelector('input[name="owner-x"]').checked ? 'x' : '-';

  const groupR = document.querySelector('input[name="group-r"]').checked ? 'r' : '-';
  const groupW = document.querySelector('input[name="group-w"]').checked ? 'w' : '-';
  const groupX = document.querySelector('input[name="group-x"]').checked ? 'x' : '-';

  const otherR = document.querySelector('input[name="other-r"]').checked ? 'r' : '-';
  const otherW = document.querySelector('input[name="other-w"]').checked ? 'w' : '-';
  const otherX = document.querySelector('input[name="other-x"]').checked ? 'x' : '-';

  const newPerms = ownerR + ownerW + ownerX + groupR + groupW + groupX + otherR + otherW + otherX;

  // Update hidden field
  const permInput = document.getElementById("permissionsInput");
  if (permInput) {
    permInput.value = newPerms;
    // Manually trigger a "change" event so HTMX sees it (if you want immediate patch)
    // or we rely on the 'delay:300ms' in hx-trigger
    permInput.dispatchEvent(new Event("change", { bubbles: true }));
  }
}

/**************************************************************************/
// after DOM content is loaded, actions, lets say initialization of page functionalities
/**************************************************************************/

// actions to do when everything is loaded
document.addEventListener("DOMContentLoaded", () => {
  /**************************************************************************/
  // sidebar
  /**************************************************************************/
  // give functionality to the sider bad of the admin panel
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

  /**************************************************************************/
  // files
  /**************************************************************************/

  // functionality of file upload via drag
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
  
  // enable/disable funcionality of file "submit" (upload) button
  function toggleSubmitButton() {
    const fileInput = document.getElementById("file");
    const submitButton = document.getElementById("upload-button");

    submitButton.disabled = fileInput.files.length === 0;
  }
  
  // handling multiple file selection/upload
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

  // file selection display
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
  // a look up to the corresponding class per given extention
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

  /**************************************************************************/
  // job setup 
  /**************************************************************************/
  
  // Job history functionalities
  // set to what we want to search by
  const jobSearchSelector = document.getElementById("job-search-by-select");

  // actual search by
  const jobSearch = document.getElementById("job-search");
  jobSearch.addEventListener("input", function() {
    // we need to search from the currently paged jobs according to the search selector
    // @TODO
  });

  // "JOB" preperation setup
  document.getElementById("language-selector").value = "python"; 

  // html/css/js mini "code editor"
  // Load CodeMirror
  const editor = CodeMirror.fromTextArea(document.getElementById("code-editor"), {
    mode: "python", // Default mode
    lineNumbers: true,
    theme: "monokai",  // Choose a theme
    matchBrackets: true,
    autoCloseBrackets: true
  });

  // Language selection logic
  document.getElementById("language-selector").addEventListener("change", function() {
    const mode = modeMap[this.value];
    editor.setOption("mode", mode);
    editor.setValue(defaultMap[this.value]);
  });

  // Code file upload logic
  document.getElementById("code-file-upload").addEventListener("change", function(event) {
    const file = event.target.files[0];
    if (!file) return;

    const ext = file.name.split('.').pop(); 

    console.log('file uploaded: ' + file.name);
    console.log('extention extracted: ' + ext)



    const mode = modeMap[ext] || "python"; 
    console.log('mode: ' + mode);
    document.getElementById("language-selector").value = extMap[ext];

    const reader = new FileReader();
    editor.setOption("mode", mode);    

    reader.onload = function(e) {
      console.log('called here');
      editor.setValue(e.target.result);
    };
    reader.readAsText(file);
  });

  // specify output functionality
  document.getElementById("select-output-destination").addEventListener("input", function(event) {
    const inputValue = event.target.value;
    const spanElement = event.target.closest('div').parentElement.nextElementSibling.children[4];
    spanElement.textContent = inputValue;
  });

  // select input "resources" for the job display handler
  document.getElementById("select-j-input-button").addEventListener("click", function(event) {
    // resource selection modal
    const existingModal = document.getElementById("resource-selection-modal");
    if (existingModal) existingModal.remove();

    // Create modal background overlay
    const modalOverlay = document.createElement("div");
    modalOverlay.id = "resource-selection-modal";
    modalOverlay.classList.add("job-select-modal-overlay")

    // Create modal content box
    const modalContent = document.createElement("div");
    modalContent.innerHTML = `
        <h3>Select Resources</h3>
        <table border="1" id="resource-selection-table" style="width:100%; border-collapse: collapse; text-align: left;">
            <thead>
                <tr>
                    <th>Select</th>
                    <th>Name</th>
                    <th>Type</th>
                    <th>Size</th>
                </tr>
            </thead>
            <tbody>
            </tbody>
        </table>
        <br>
        <button id="submit-resource-selection">Submit</button>
        <button id="cancel-resource-selection">Cancel</button>
    `;

    modalOverlay.appendChild(modalContent);
    document.body.appendChild(modalOverlay);

    // Reference existing resources from a previous table
    const selectionTableBody = modalContent.querySelector("tbody");

    const cachedResources = new Promise((resolve, reject) => {
      fetch('/api/v1/verified/admin/fetch-resources?format=json')
        .then(response => {
          if (!response.ok) {
            throw new Error('Network response was not ok');
          }
          return response.json();
        })
        .then(data => resolve(data))
        .catch(error => reject(error));
    });

    cachedResources.then(resources => {
      resources.forEach((resource) => {
        const resourceId = resource.rid;
        const resourceName = resource.name;
        const resourceType = resource.type;
        const resourceSize = resource.size;

        const newRow = document.createElement("tr");
        newRow.innerHTML = `
          <td><input type="checkbox" data-resource-id="${resourceId}" data-resource-name="${resourceName}"></td>
          <td>${resourceName}</td>
          <td>${resourceType}</td>
          <td>${resourceSize}</td>
        `;
        selectionTableBody.appendChild(newRow);
      });
    }).catch(error => {
      console.error('Error fetching resources:', error);
    });

    /*if (cachedResources) {
      cachedResources.forEach((resource) => {
            const resourceId = resource.rid;
            const resourceName = resource.name;
            const resourceType = resource.type;
            const resourceSize = resource.size;

            const newRow = document.createElement("tr");
            newRow.innerHTML = `
                <td><input type="checkbox" data-resource-id="${resourceId}" data-resource-name="${resourceName}"></td>
                <td>${resourceName}</td>
                <td>${resourceType}</td>
                <td>${resourceSize}</td>
            `;
            selectionTableBody.appendChild(newRow);
        });
    }
    */
    // Handle submission
    document.getElementById("submit-resource-selection").addEventListener("click", function () {
        const selectedResources = [];
        document.querySelectorAll("#resource-selection-table input[type='checkbox']:checked").forEach((checkbox) => {
            selectedResources.push({
                id: checkbox.getAttribute("data-resource-id"),
                name: checkbox.getAttribute("data-resource-name"),
            });
        });

        console.log("Selected Resources:", selectedResources);

        // You can send selectedResources to another function or API
        //alert(`Selected ${selectedResources.length} resources!`);
        document.querySelector(".input-box").textContent = selectedResources.map(resource => `${resource.name}`).join('\n');

        // Close the modal
        modalOverlay.remove();
    });

    // Handle cancel
    document.getElementById("cancel-resource-selection").addEventListener("click", function () {
        modalOverlay.remove();
    });



  });

  setTimeout(() => {
    editor.setValue(defaultMap["python"] || "");  // Prevent undefined values
    
    document.getElementById("submit-job-button").checked = true;
  }, 100);

  // Job submission, lets do it as a promise, more flexible for this rather than htmx
  const submitJobBtn = document.getElementById("submit-job-button");

  submitJobBtn.addEventListener("change", function(event) {
    if (submitJobBtn.checked) {// cancel job case {}
      //confirm?
      if (!confirm("Are you sure you want to cancel the job execution?")) {
        submitJobBtn.checked = false;
        return;
      }
  
      // start an indicator spinner
      const jloader = document.querySelector('.j-loader');
      jloader.classList.remove("hidden");
      jloader.style.animation="reverseSpin var(--speed) infinite linear";
      // send the request and await response (maybe trigger a ws to get realtime data about the job)
      // handle response, display, spinner, output

    
    } else { // send job

      // start an indicator spinner
      const jloader = document.querySelector('.j-loader');
      jloader.style.animation="spin var(--speed) infinite linear";
      jloader.classList.remove("hidden");
      
      const input = document.querySelector(".input-box").textContent.split('\n').map((file) => file.replace(/^\/+/, ''));
      const output = document.querySelector(".output-box").textContent;
      const code = editor.getValue();
      const logic = editor.getOption("mode");


      // verify logic integrity
      // gather data
      let job = {
        "uid":0,
        "input":input,
        "output":output,
        "logic":logic,
        "logic_body":code,
      }

      console.log(job);

      // send the request and await response 
      const response = new Promise((resolve, reject) => {
        fetch('/api/v1/verified/jobs', {
          method: 'POST',
          headers: {
        'Content-Type': 'application/json',
          },
          body: JSON.stringify(job),
        })
          .then(response => {
        if (!response.ok) {
          throw new Error('Network response was not ok');
        }
        return response.json();
          })
          .then(data => resolve(data))
          .catch(error => reject(error));
      });

      // handle response, display, spinner, output, (maybe trigger a ws on success to get realtime data about the job)
      response.then(resp => {
        console.log(resp);
      }).catch(error => {
        console.error('Error fetching resources:', error);
      });

      setTimeout(() => {
        jloader.classList.add('hidden');
        submitJobBtn.checked = true;
      }, 2000);

    }
  });

  /**************************************************************************/
  /**************************************************************************/

});


