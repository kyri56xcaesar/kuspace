/* code specific that only admin panel should use. */

/**************************************************************************/
// global variables/constants
/**************************************************************************/



let fileUploadModule;


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

    if (i == 6) {
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
  let ed4 = document.getElementById("edit-input-"+uid+"-4")
  let ed5 = document.getElementById("edit-input-"+uid+"-5");
  let ed7 = document.getElementById("edit-input-"+uid+"-7");
  r = {
    uid: uid,
    username : ed1.value,
    password : ed2.value,
    info : ed3.value,
    home: ed4.value,
    shell: ed5.value,
    groups: ed7.value
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
  const submitButton = document.getElementById("upload-button");
  const fileNameDisplay = document.getElementById("file-name");

  fileUploadModule = fileUploadContainerFunctionality(
    dropZone,
    fileInput,
    fileBoxContainer,
    submitButton,
    fileNameDisplay
  );
 

  /**************************************************************************/
  // job setup 
  /**************************************************************************/
  
  // Job history functionalities
  // set to what we want to search by
  searchBy = "created_at";
  const jobSearchSelector = document.getElementById("job-search-by-select");
  jobSearchSelector.value = searchBy;
  jobSearchSelector.addEventListener("input", (event) => {
    searchBy = jobSearchSelector.value;
  });

  // actual search by
  const jobSearch = document.getElementById("job-search");
  jobSearch.value = "";
  jobSearch.addEventListener("input", function() {
    // we need to search from the currently paged jobs according to the search selector
    // @TODO
    searchValue = jobSearch.value;
    // console.log("searching by " + searchBy + " at " + searchValue);
    if (cacheJobResultsLi.length == 0) {// empty cache, must fetch 
      
    }

    // do search and display
    switch (searchBy) {
      case "jid":
        cacheJobResultsLi.forEach((li) => {
          const jidSpan = li.querySelector(".jid");

          if (!jidSpan.innerText.includes(searchValue)) {
            li.classList.add("hidden");
          } else {
            li.classList.remove("hidden");
          }
        });
        break;
      case "uid":
        cacheJobResultsLi.forEach((li) => {
          if (!li.querySelector(".uid").innerText.includes(searchValue)) {
            li.classList.add("hidden");
          } else {
            li.classList.remove("hidden");
          }
        });
        break;
      case "created_at":
        cacheJobResultsLi.forEach((li) => {
          if (!li.querySelector(".created_at").innerText.includes(searchValue)) {
            li.classList.add("hidden");
          } else {
            li.classList.remove("hidden");
          }
        });
        break;
      case "completed_at":
        cacheJobResultsLi.forEach((li) => {
          if (!li.querySelector(".completed_at").innerText.includes(searchValue)) {
            li.classList.add("hidden");
          } else {
            li.classList.remove("hidden");
          }
        });
        break;
      case "status":
        cacheJobResultsLi.forEach((li) => {
          if (!li.querySelector(".status").innerText.includes(searchValue)) {
            li.classList.add("hidden");
          } else {
            li.classList.remove("hidden");
          }
        });
        break;    
      case "output":
        cacheJobResultsLi.forEach((li) => {
          if (!li.querySelector(".output").innerText.includes(searchValue)) {
            li.classList.add("hidden");
          } else {
            li.classList.remove("hidden");
          }
        });
        break;
      default:
         break;
    }
  });

  setupJobSubmitter(document.querySelector('#new-job-container'));
  
  /**************************************************************************/
  /**************************************************************************/

});


