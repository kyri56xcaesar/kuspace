/* code specific that only admin panel should use. */

/**************************************************************************/
// global variables/constants
/**************************************************************************/
cachedUsers = [];
cachedGroups = [];
cachedResources = [];


const PORT = window.location.port || (window.location.protocol === "https:" ? "443" : "80");
const WS_PORT = "8082"
const IP = window.location.hostname; 
let fileUploadModule;


let WS_ADDRESS = null;
async function initWebSocketAddress() {
  try {
    const response = await fetch("/conf");
    if (!response.ok) {
      throw new Error(`Failed to fetch config: ${response.status}`);
    }

    const config = await response.json();
    if (!config.ws_address) {
      throw new Error("WebSocket address not provided in config");
    }

    const port = config.ws_address.trim().split(":")[1]
    WS_ADDRESS = IP + ":" + port;
    console.log("WS_ADDRESS set to:", WS_ADDRESS);
  } catch (err) {
    console.error("Error setting WS_ADDRESS:", err);
    WS_ADDRESS = null;
  }
}
(async () => {
  await initWebSocketAddress();

  if (!WS_ADDRESS) {
    console.error("WebSocket address not initialized.");
    return;
  }

  // Use socket here...
})();


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
  let ed4 = document.getElementById("edit-input-"+uid+"-4")
  let ed6 = document.getElementById("edit-input-"+uid+"-6");
  r = {
    uid: uid,
    username : ed1.value,
    password : ed2.value,
    info : ed3.value,
    home: ed4.value,
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

function toggle_job_optionals(jobDiv) {
  if (jobDiv) {
    const jobOptionals = jobDiv.querySelectorAll(".job-optional");
    jobOptionals.forEach((optional) => {
      optional.classList.toggle("hidden");
    });
  }
}



/**************************************************************************/
// after DOM content is loaded, actions, lets say initialization of page functionalities
/**************************************************************************/

// actions to do when everything is loaded
document.addEventListener("DOMContentLoaded", () => {
  /**************************************************************************/
  // side-bar
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
  
  setupJobSubmitter(document.querySelector('#job-create-form-editor'));

  // select a resource input for the job
  const job_input = document.getElementById("job-input")
  job_input.value = ""; //reset the value
  document.getElementById("select-resource-btn-job").addEventListener("click", () => {
    const modal = document.getElementById("select-resource-btn-job").parentNode.querySelector(".modal");
    const resourceList = modal.querySelector("#resource-list");
    resourceList.innerHTML = "";

    
    // must retrieve all the resources from the page somehwere..
    const rTable = document.querySelector("#resource-list-table > tbody");
    // if not admin you load them from a differnt place
    if (!rTable) {
      if (cachedResources) {
        cachedResources.forEach((li) => {
          let rname = li.name;
          rname = rname.startsWith("/") ? rname : "/" + rname
          const resourceName =  li.vname + rname;
    
          const button = document.createElement("button");
          button.textContent = resourceName;
          button.type = "button";
          button.addEventListener("click", () => {
            job_input.value = resourceName;
            document.getElementById("select-resource-btn-job").parentNode.querySelector(".modal").classList.add("hidden");
          });
          resourceList.appendChild(button);
        });
      } else {
        // we need to fetch the resources...
      }
      modal.classList.remove("hidden");
    } else {
      rTable.querySelectorAll("tr").forEach((li) => {
        let rname = li.querySelector(".name").textContent.trim();
        rname = rname.startsWith("/") ? rname : "/" + rname
        let vname =li.querySelector(".volume").textContent.trim();
        const resourceName =  vname + rname;
  
        const button = document.createElement("button");
        button.textContent = resourceName;
        button.type = "button";
        button.addEventListener("click", () => {
          job_input.value = resourceName;
          document.getElementById("select-resource-btn-job").parentNode.querySelector(".modal").classList.add("hidden");
        });
        resourceList.appendChild(button);
      });
      modal.classList.remove("hidden");
    }


  });


  // select a volume output for the job
  const job_output = document.getElementById("job-output");
  job_output.value = ""; //reset the value
  document.getElementById("select-volume-btn-job").addEventListener("click", () => {
    const modal =  document.getElementById("select-volume-btn-job").parentNode.querySelector(".modal");
    const volumeList = modal.querySelector("#volume-list");

    // Clear existing list
    volumeList.innerHTML = "";

    // Grab all volumes from the page
    document.querySelectorAll(".v-body h3").forEach((h3) => {
      const volumeName = h3.textContent.trim();

      const button = document.createElement("button");
      button.textContent = volumeName;
      button.type = "button";
      button.addEventListener("click", () => {
        job_output.value = volumeName + "/";
        document.getElementById("select-volume-btn-job").parentNode.querySelector(".modal").classList.add("hidden");
      });

      volumeList.appendChild(button);
    });

    modal.classList.remove("hidden");
  });


  
  /**************************************************************************/
  /**************************************************************************/
  // Volumes
  /**************************************************************************/
  /**************************************************************************/
  const vSearch = document.getElementById("volume-search");
  let vSearchBy = "name";
  const vSearchSelector = document.querySelector(".v-header").querySelector("#search-by");
  vSearchSelector.value = vSearchBy;
  vSearchSelector.addEventListener("input", (event) => {
    vSearchBy = vSearchSelector.value;
    vSearch.placeholder = "Search by '" + vSearchBy+"'";
  });

  vSearch.value = "";
  vSearch.addEventListener("input", function() {
    if (cacheVolumeResults.length == 0) {// empty cache, must fetch 
      
    }
    searchValue = vSearch.value;
    // console.log("searching by " + searchBy + " at " + searchValue);
    // do search and display
    cacheVolumeResults.forEach((li) => {
      switch (vSearchBy) {
        case "name":
          if (!li.querySelector(".name").innerText.includes(searchValue)) {
            li.classList.add("hidden");
          } else {
            li.classList.remove("hidden");
          }
          break;
        case "createdat":
          if (!li.querySelector(".createdat").innerText.includes(searchValue)) {
            li.classList.add("hidden");
          } else {
            li.classList.remove("hidden");
          }
          break;
      }
    });
 
  });

  const cancelModalbtn = document.getElementById("cancel-modal-btn");
  if (cancelModalbtn) {
    cancelModalbtn.addEventListener("click", () => {
      document.getElementById("create-volume-modal").classList.add("hidden");
    });    
  }

  // for choosing a volume when upload
  document.getElementById("select-volume-btn").addEventListener("click", () => {
    const modal = document.getElementById("select-volume-btn").parentNode.querySelector(".modal");
    const volumeList = modal.querySelector("#volume-list");

    // Clear existing list
    volumeList.innerHTML = "";

    // Grab all volumes from the page
    document.querySelectorAll(".v-body h3").forEach((h3) => {
      const volumeName = h3.textContent.trim();

      const button = document.createElement("button");
      button.textContent = volumeName;
      button.type = "button";
      button.addEventListener("click", () => {
        document.getElementById("selected-volume").textContent = volumeName;
        document.getElementById("select-volume-btn").parentNode.querySelector(".modal").classList.add("hidden");
      });

      volumeList.appendChild(button);
    });

    modal.classList.remove("hidden");
  });
  

  document.querySelectorAll("#cancel-select").forEach((cancel_btn) => {
    cancel_btn.addEventListener("click", () => {
      cancel_btn.parentNode.parentNode.classList.add("hidden");
    });
    
  })



  /**************************************************************************/
  /**************************************************************************/
  // Resources
  /**************************************************************************/
  /**************************************************************************/
  const rSearch = document.getElementById("resource-search");
  if (rSearch) {
    let rSearchBy = "name";
    const rSearchBySelector = document.getElementById("resources-header").querySelector("#search-by");
    rSearchBySelector.value = rSearchBy;
    rSearchBySelector.addEventListener("input", (event) => {
      rSearchBy = rSearchBySelector.value;
      rSearch.placeholder = "Search by '" + rSearchBy+"'";
    });
    rSearch.value = "";
    rSearch.addEventListener("input", function() {
      if (cacheResourceResults.length == 0) {// empty cache, must fetch 
        
      }
      searchValue = rSearch.value;
      // console.log("searching by " + searchBy + " at " + searchValue);
      // do search and display
      let r_dets = document.getElementById("resource-details");
      cacheResourceResults.forEach((li) => {
        switch (rSearchBy) {
          case "name":
            if (!li.querySelector(".name").innerText.includes(searchValue)) {
              li.classList.add("hidden");
            } else {
              li.classList.remove("hidden");
              parseAndInjectRTableRowdata(li, r_dets);
            }
            break;
          case "volume":
            if (!li.querySelector(".volume").innerText.includes(searchValue)) {
              li.classList.add("hidden");
            } else {
              li.classList.remove("hidden");
              parseAndInjectRTableRowdata(li, r_dets);
            }
            break;
          case "createdat":
            if (!li.querySelector(".createdat").innerText.includes(searchValue)) {
              li.classList.add("hidden");
            } else {
              li.classList.remove("hidden");
              parseAndInjectRTableRowdata(li, r_dets);
            }
            break;
          case "updatedat":
            if (!li.querySelector(".updatedat").innerText.includes(searchValue)) {
              li.classList.add("hidden");
            } else {
              li.classList.remove("hidden");
              parseAndInjectRTableRowdata(li, r_dets);
            }
            break;
          case "accessedat":
            if (!li.querySelector(".accessedat").innerText.includes(searchValue)) {
              li.classList.add("hidden");
            } else {
              li.classList.remove("hidden");
              parseAndInjectRTableRowdata(li, r_dets);
            }
            break;
          case "owner":
            if (!li.querySelector(".owner").innerText.includes(searchValue)) {
              li.classList.add("hidden");
            } else {
              li.classList.remove("hidden");
              parseAndInjectRTableRowdata(li, r_dets);
            }
            break;
          case "group":
            if (!li.querySelector(".group").innerText.includes(searchValue)) {
              li.classList.add("hidden");
            } else {
              li.classList.remove("hidden");
              parseAndInjectRTableRowdata(li, r_dets);
            }
            break;
        }
  
      });
  
  
      
    });
  }


});

function addResourceListListeners() {
  tableRows = document.querySelectorAll("#resource-list-table tbody tr");
  resourceDetails = document.getElementById("resource-details");

  tableRows.forEach((row) => {
    row.addEventListener("click", () => {
      // Remove 'selected' class from all rows
      tableRows.forEach((r) => r.classList.remove("selected"));
      // Add 'selected' class to the clicked row
      row.classList.add("selected");
      // Extract resource information from the row

      parseAndInjectRTableRowdata(row, resourceDetails);
      
      const btns = document.querySelectorAll(".r-btn-download, .r-btn-edit, .r-btn-delete, #preview-resource-btn, #next-arrow-right, #next-arrow-left");
      btns.forEach(button => {
        htmx.process(button);
      });

    });
  });
}

function parseAndInjectRTableRowdata(tr, injectTarget) {
  if (!tr || tr.cells.length != 13 || !injectTarget) {
    return
  }

  const resource = {
    id: tr.cells[0].innerText,
    name: tr.cells[1].innerText,
    path: tr.cells[2].innerText,
    vname: tr.cells[3].innerText,
    type: tr.cells[4].innerText,
    size: tr.cells[5].innerText,
    perms: tr.cells[6].innerText,
    createdAt: tr.cells[7].innerText,
    updatedAt: tr.cells[8].innerText,
    accessedAt: tr.cells[9].innerText,
    owner: tr.cells[10].innerText,
    group: tr.cells[11].innerText,
    vid: tr.cells[12].innerText,
  };
  
  // injection
  injectTarget.innerHTML = `
      <div class="resource-details-headers">
        <h3>Resource Details</h3>
        <div class="resource-options">
          <i id="resource-options-dropdown-button" onclick="this.nextElementSibling.firstElementChild.classList.toggle('open');" class="fa">&#xf078;</i>
          <div class="resource-options">
            <div class="resource-options-inner dropdown">
              <button 
                class="r-btn-download" 
                onclick="downloadResource('/api/v1/verified/download?target=${resource.name}&volume=${resource.vname}')"
              >
                Download
              </button>

              <button
                id="preview-resource-btn"
                hx-target="#resource-preview-content-1"
                hx-trigger="click"
                hx-swap="innerHTML"
                hx-get="/api/v1/verified/preview?rid=${resource.id}&resourcename=${resource.name}&volume=${resource.vname}"
                hx-headers='{"Range": "bytes=${getPreviewWindow(0)}"}'
              >Preview</button>

              <button 
                class="r-btn-edit"
                hx-get="/api/v1/verified/edit-form?resourcename=${resource.name}&owner=${resource.owner || 0}&group=${resource.group || 0}&perms=${resource.perms}&rid=${resource.id}&volume=${resource.vname}"
                hx-swap="innerHTML"
                hx-trigger="click"
                hx-target="#edit-modal-2"
                hx-on::after-request="show(this.parentNode.querySelector('#edit-modal-2'))"
                >
                Edit
              </button>
              <div id="edit-modal-2" class="modal hidden darkened"></div>

              <button 
                class="r-btn-delete"
                hx-delete="/api/v1/verified/rm?name=${resource.name}&volume=${resource.vname}"
                hx-trigger="click"
                hx-swap="none"
                hx-confirm="Are you sure you want to delete resource ${resource.name}?"

                hx-on::before-request="show(document.querySelector('.r-loader'))"
              >
                Delete
              </button>

              <button
                id="close-r-selected-display"
                onclick="document.getElementById('resource-details').innerHTML='';"
              >
              Close
              </button>
            </div>
          </div>
        </div>
      </div>
      <hr>
      <div class="resource-details-main">
        <div class="resource-details-inner">
          <p><strong>Rid:</strong> ${resource.id}</p>
          <p><strong>Name:</strong> ${resource.name}</p>
          <p><strong>Path:</strong> ${resource.path}</p>
          <p><strong>Volume:</strong> ${resource.vname}</p>
          <p><strong>Type:</strong> ${resource.type}</p>
          <p><strong>Size:</strong> ${resource.size}</p>
          <p><strong>Permissions:</strong> ${resource.perms}</p>
          <p><strong>Created At:</strong> ${resource.createdAt}</p>
          <p><strong>Updated At:</strong> ${resource.updatedAt}</p>
          <p><strong>Accessed At:</strong> ${resource.accessedAt}</p>
          <p><strong>Owner:</strong> ${resource.owner}</p>
          <p><strong>Group:</strong> ${resource.group}</p>
          <p><strong>Volume:</strong> ${resource.vname}</p>
        </div>
        <div id="resource-preview" class="resource-preview-window">
          <div class="resource-preview-main blurred">
            <div id="resource-preview-content-1" class="resource-preview-content"></div>
            <div id="resource-preview-controls">
              <div 
                id="next-arrow-left" 
                class="next-arrow"
                hx-target="#resource-preview-content-1"
                hx-trigger="click"
                hx-swap="innerHTML"
                hx-get="/api/v1/verified/preview?rid=${resource.id}&resourcename=${resource.name}&volume=${resource.vname}"
                hx-headers='js:{"Range": "bytes=" + getPreviewWindow(-1)}'  
              >
                <svg width="24" height="8" viewBox="0 0 16 8" fill="none" xmlns="http://www.w3.org/2000/svg" class="arrow-icon">
                <g transform="scale(-1,1) translate(-16,0)">
                  <path d="M15 4H4V1" stroke="white"/>
                  <path d="M14.5 4H3.5H0" stroke="white"/>
                  <path d="M15.8536 4.35355C16.0488 4.15829 16.0488 3.84171 15.8536 3.64645L12.6716 0.464466C12.4763 0.269204 12.1597 0.269204 11.9645 0.464466C11.7692 0.659728 11.7692 0.976311 11.9645 1.17157L14.7929 4L11.9645 6.82843C11.7692 7.02369 11.7692 7.34027 11.9645 7.53553C12.1597 7.7308 12.4763 7.7308 12.6716 7.53553L15.8536 4.35355ZM15 4.5L15.5 4.5L15.5 3.5L15 3.5L15 4.5Z" fill="white"/>
                </g>
                </svg>
              </div>
              <div>
                <span id="page-index">0</span>
              </div>
              <div 
                id="next-arrow-right" 
                class="next-arrow"
                hx-target="#resource-preview-content-1"
                hx-trigger="click"
                hx-swap="innerHTML"
                hx-get="/api/v1/verified/preview?rid=${resource.id}&resourcename=${resource.name}&volume=${resource.vname}"
                hx-headers='js:{"Range": "bytes=" + getPreviewWindow(+1)}'  
              >
                <svg width="24" height="8" viewBox="0 0 16 8" fill="none" xmlns="http://www.w3.org/2000/svg" class="arrow-icon">
                  <path d="M15 4H4V1" stroke="white"/>
                  <path d="M14.5 4H3.5H0" stroke="white"/>
                  <path d="M15.8536 4.35355C16.0488 4.15829 16.0488 3.84171 15.8536 3.64645L12.6716 0.464466C12.4763 0.269204 12.1597 0.269204 11.9645 0.464466C11.7692 0.659728 11.7692 0.976311 11.9645 1.17157L14.7929 4L11.9645 6.82843C11.7692 7.02369 11.7692 7.34027 11.9645 7.53553C12.1597 7.7308 12.4763 7.7308 12.6716 7.53553L15.8536 4.35355ZM15 4.5L15.5 4.5L15.5 3.5L15 3.5L15 4.5Z" fill="white"/>
                </svg>
              </div>
            </div>
          </div>
        </div>
        <div id="edit-modal" class="modal hidden"></div>
      </div>
      <hr>
      <div class="resource-details-footer">
        <div class="feedback"></div>
        <div class="r-loader hidden"><div></div></div>
      </div>`;


}

function setupSearchBar(jobSearchDiv, cacheJobResults) {
  let searchBy = "jid";
  const jobSearch = jobSearchDiv.querySelector("#job-search");
  const jobSearchSelector = jobSearchDiv.querySelector("#search-by");
  jobSearchSelector.value = searchBy;
  jobSearchSelector.addEventListener("input", (event) => {
    searchBy = jobSearchSelector.value;
    jobSearch.placeholder = "Search by '" + searchBy+"'";
  });

  // actual search by
  jobSearch.value = "";
  jobSearch.addEventListener("input", function() {
    // we need to search from the currently paged jobs according to the search selector
    // @TODO
    searchValue = jobSearch.value;
    // console.log("searching by " + searchBy + " at " + searchValue);
    if (cacheJobResults.length == 0) {// empty cache, must fetch 
      
    }

    // do search and display
    switch (searchBy) {
      case "jid":
        cacheJobResults.forEach((li) => {
          const jidSpan = li.querySelector(".jid");

          if (!jidSpan.innerText.includes(searchValue)) {
            li.classList.add("hidden");
          } else {
            li.classList.remove("hidden");
          }
        });
        break;
      case "uid":
        cacheJobResults.forEach((li) => {
          if (!li.querySelector(".uid").innerText.includes(searchValue)) {
            li.classList.add("hidden");
          } else {
            li.classList.remove("hidden");
          }
        });
        break;
      case "createdAt":
        cacheJobResults.forEach((li) => {
          if (!li.querySelector(".createdAt").innerText.includes(searchValue)) {
            li.classList.add("hidden");
          } else {
            li.classList.remove("hidden");
          }
        });
        break;
      case "completed_at":
        cacheJobResults.forEach((li) => {
          if (!li.querySelector(".completed_at").innerText.includes(searchValue)) {
            li.classList.add("hidden");
          } else {
            li.classList.remove("hidden");
          }
        });
        break;
      case "status":
        cacheJobResults.forEach((li) => {
          if (!li.querySelector(".status").innerText.includes(searchValue)) {
            li.classList.add("hidden");
          } else {
            li.classList.remove("hidden");
          }
        });
        break;    
      case "output":
        cacheJobResults.forEach((li) => {
          if (!li.querySelector(".output").innerText.includes(searchValue)) {
            li.classList.add("hidden");
          } else {
            li.classList.remove("hidden");
          }
        });
      case "input":
        cacheJobResults.forEach((li) => {
          if (!li.querySelector(".input").innerText.includes(searchValue)) {
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
}

function modJobModal(div, parentDiv) {
  if (!div || !parentDiv) {
    return
  }
  const jid = div.querySelector('.jid').textContent.replace("#JobId:", "").trim();
  const uid = div.querySelector('.uid').textContent.replace("by", "").trim();
  const status = div.querySelector('.status').textContent.replace("Status:", "").trim();
  const duration = div.querySelector('.duration').textContent.replace("Duration: ", "").trim();
  const input = div.querySelector('.input').textContent.replace("Input: ", "").trim();
  const output = div.querySelector('.output').textContent.replace("Output: ", "").trim();
  const description = div.querySelector('.description').textContent.replace("Description:", "").trim();
  const createdAt = div.querySelector('.createdAt').textContent.replace('CreatedAt:', '').trim();
  const completed_at = div.querySelector('.completed_at').textContent.replace('CompletedAt:', '').trim();
  const completed = div.querySelector('.completed').textContent.replace("Completed:", "").trim();
  const timeout = div.querySelector('.timeout').textContent.trim();
  const parallelism = div.querySelector('.parallelism').textContent.trim();
  const priority = div.querySelector('.priority').textContent.trim();
  const memory_request = div.querySelector('.memory_request').textContent.trim();
  const cpu_request = div.querySelector('.cpu_request').textContent.trim();
  const memory_limit = div.querySelector('.memory_limit').textContent.trim();
  const cpu_limit = div.querySelector('.cpu_limit').textContent.trim();
  const ephimeral_storage_request = div.querySelector('.ephimeral_storage_request').textContent.trim();
  const ephimeral_storage_limit = div.querySelector('.ephimeral_storage_limit').textContent.trim();
  const logic = div.querySelector('.logic').textContent.trim();
  const logic_body = div.querySelector('.logic_body').textContent.trim();
  const logic_headers = div.querySelector('.logic_headers').textContent.trim();

  const html = `
  <div class="modal-content">
    <h2>Modify Job</h2>
    <form 
      id="modify-job-form"
      hx-put="/api/v1/verified/admin/jobs"
      hx-swap="none"
      hx-trigger="submit"
    >
    <div class="disabled-display">
      <div>
        Job id
        <input type="number" name="jid" value="${jid}" readonly="readonly">
      </div><br>

      <div>
        Duration
        <input type="text" name="duration" value="${duration}" readonly="readonly">
      </div><br>
      <div>
        Input
        <input type="text" name="input" value="${input}" readonly="readonly">
      </div><br>
      <div>
        Output
        <input type="text" name="output" value="${output}" readonly="readonly">
      </div><br>
      <div>
        CreatedAt
        <input type="text" name="createdAt" value="${createdAt}" readonly="readonly">
      </div><br>
      <div>
        CompletedAt
        <input type="text" name="completed_at" value="${completed_at}" readonly="readonly">
      </div><br>
      <div>
        <input type="text" name="logic" value="${logic}" hidden>
        <input type="text" name="logic_body" value="${logic_body}" hidden>
        <input type="text" name="logic_headers" value="${logic_headers}" hidden>
        <input type="text" name="ephimeral_storage_limit" value="${ephimeral_storage_limit}" hidden>
        <input type="text" name="ephimeral_storage_request" value="${ephimeral_storage_request}" hidden>
        <input type="text" name="cpu_limit" value="${cpu_limit}" hidden>
        <input type="text" name="memory_limit" value="${memory_limit}" hidden>
        <input type="text" name="cpu_request" value="${cpu_request}" hidden>
        <input type="text" name="memory_request" value="${memory_request}" hidden>
        <input type="number" name="priority" value="${priority}" hidden>
        <input type="number" name="parallelism" value="${parallelism}" hidden>
        <input type="number" name="timeout" value="${timeout}" hidden>
      </div>
    </div>
      <div>
        Description<br>
        <textarea name="description" maxlength="150">${description}</textarea>
      </div><br>

      <div>
        User id<br>
        <input type="number" name="uid" value="${uid}" min="0" required>
      </div><br>

      <div>
        Status <br>
        <input type="text" name="status" value="${status}" required>
      </div><br>

      <div>
        Completed
        <input type="checkbox" name="completed" value="true" ${completed === "true" || completed === true ? "checked" : ""}>
      </div><br>

      <div class="modal-actions">
        <button type="submit">Submit</button>
        <button type="button" id="cancel-modal-btn" onclick="this.parentNode.parentNode.parentNode.parentNode.classList.add('hidden');">Cancel</button>
      </div>
    </form>
  </div>
  <div class="feedback hidden modal-feedback"></div>
  `;

  const modalExists = parentDiv.querySelector('.modal');
  if (modalExists) {
    modalExists.innerHTML = html;
    modalExists.classList.remove('hidden');
    htmx.process(modalExists.querySelector("#modify-job-form"));
  } else {
    const modJobDiv = document.createElement("div");
    modJobDiv.className = "modal";
    modJobDiv.innerHTML = html;
    htmx.process(modJobDiv.querySelector("#modify-job-form"));
    parentDiv.appendChild(modJobDiv);
  }
}

function modAppModal(div, parentDiv) {
   if (!div || !parentDiv) {
    return
  }
  const id = div.querySelector('.app-id').textContent.replace("app-id:", "").trim();
  const name = div.querySelector('.app-name').textContent.trim().split(" ")[0];
  const version = div.querySelector('.app-version').textContent.replace("v", "").trim();
  const status = div.querySelector('.app-status').textContent.trim();
  const image = div.querySelector('.image').textContent.replace("Image:", "").trim();
  const author = div.querySelector('.author').textContent.replace("Author: ", "").trim();
  const author_id = div.querySelector('.author_id').textContent.trim();
  const createdAt = div.querySelector('.createdAt').textContent.replace("Created: ", "").trim();
  const insertedAt = div.querySelector('.insertedAt').textContent.trim();
  const description = div.querySelector('.app-description').textContent.trim();
 
  const html = `
  <div class="modal-content">
    <h2>Modify App</h2>
    <form 
      id="modify-app-form"
      class="modify-app"
      hx-put="/api/v1/verified/admin/apps"
      hx-swap="none"
      hx-trigger="submit"
    >
      <div class="disabled-display">
        <div>
          App id
          <input type="number" name="id" value="${id}" readonly="readonly"  tabindex="-1">
        </div><br>
        <div>
          CreatedAt
          <input type="text" name="createdAt" value="${createdAt}" readonly="readonly"  tabindex="-1">
        </div><br>
        <div>
          Inserted_at
          <input type="text" name="insertedAt" value="${insertedAt}" readonly="readonly"  tabindex="-1">
        </div><br>
        <div>
          <input type="hidden" name="author_id" value="${author_id}"  tabindex="-1">
        </div>
        
      </div>
      <div>
        Name<br>
        <input type="text" name="name" value="${name}" required>
      </div><br>
      <div>
        Image<br>
        <input type="text" name="image" value="${image}" required>
      </div><br>
      <div>
        Version<br>
        <input type="text" name="version" value="${version}" required>
      </div><br>
      <div>
        Author<br>
        <input type="text" name="author" value="${author}" required>
      </div><br>

      <div>
        Description<br>
        <textarea name="description" maxlength="150">${description}</textarea>
      </div><br>

      <div>
        Status <br>
        <input type="text" name="status" value="${status}" required>
      </div><br>


      <div class="modal-actions">
        <button type="submit">Submit</button>
        <button type="button" id="cancel-modal-btn" onclick="this.parentNode.parentNode.parentNode.parentNode.classList.add('hidden');">Cancel</button>
      </div>
    </form>
  </div>
  <div class="feedback hidden modal-feedback"></div>
  `;

  const modalExists = parentDiv.querySelector('.modal');
  if (modalExists) {
    modalExists.innerHTML = html;
    modalExists.classList.remove('hidden');
    htmx.process(modalExists.querySelector("#modify-app-form"));
  } else {
    const modJobDiv = document.createElement("div");
    modJobDiv.className = "modal";
    modJobDiv.innerHTML = html;
    htmx.process(modJobDiv.querySelector("#modify-app-form"));
    parentDiv.appendChild(modJobDiv);
  }
}
