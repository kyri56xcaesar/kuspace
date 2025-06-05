vfsRoot = {};
currentPath = [];
// Build the VFS tree from paths
// function buildTree(resources) {
//   // console.log(paths);
//   const root = {};
//   resources.forEach(resource => {
//     console.log('resource: ', resource);
//     const parts = resource.name.split("/").filter(Boolean);
//     // console.log(parts);
//     let node = root;
//     parts.forEach((part, index) => {
//       if (!node[part]) {
//         node[part] = {
//           __isFile: (index === parts.length - 1) && (parts[parts.length - 1] != "."),
//           __children: {}
//         };
//       }
//       node = node[part].__children;
//     });
//   });
//   return root;
// }

function buildTree(resources) {
  const root = {};

  resources.forEach(resource => {
    const volumeRoot = resource.vname || "default";
    const parts = [volumeRoot, ...resource.name.split("/").filter(Boolean)];

    let node = root;
    parts.forEach((part, index) => {
      if (!node[part]) {
        node[part] = {
          __isFile: (index === parts.length - 1),
          __children: {}
        };
      }
      node = node[part].__children;
    });
  });

  return root;
}


function getNodeAtPath(pathParts) {
  let node = vfsRoot;
  for (const part of pathParts) {
    if (node[part]) {
      node = node[part].__children;
    } else {
      return null;
    }
  }
  return node;
}
  
function renderVFS(pathParts, container) {
  container.innerHTML = "";

  const node = getNodeAtPath(pathParts);
  if (!node) return;

  if (pathParts.length > 0) {
    const back = document.createElement("div");
    back.textContent = "..";
    back.classList.add("back");
    back.style.cursor = "pointer";
    back.onclick = () => {
      currentPath.pop();
      renderVFS(currentPath, container);
    };
    container.appendChild(back);
  }

  Object.keys(node).sort().forEach(key => {
    const entry = node[key];
    const isFile = entry.__isFile;
    const div = document.createElement("div");
    div.textContent = (isFile || key == ".") ? key : key + "/";
    div.classList.add(isFile ? "file" : "directory");
    div.style.cursor = "pointer";
    div.style.paddingLeft = "10px";
    div.onclick = () => {
      if (isFile) {
        displaySelectedResource([...currentPath, key].join("/"));
      } else {
        if (key != ".") {
          currentPath.push(key);
          renderVFS(currentPath, container);
        }
      }
    };
    container.appendChild(div);
  });
}

function displaySelectedResource(resourcePath) {
  const targetDiv = document.getElementById("selected-resource-display");
  targetDiv.classList.remove("hidden");
  // find resource data by name (the)

  let resource;

  for (resource of cachedResources) {
    if (resourcePath.includes(resource.name)) {
      resource = resource;
      break;
    }
  }

  // Update the resource details div
  targetDiv.innerHTML = `
    <div class="resource-details-headers">
      <h3>details</h3>
      <div class="resource-options">
        <i id="resource-options-dropdown-button" style="font-size:24px" onclick="this.nextElementSibling.firstElementChild.classList.toggle('open');" class="fa">&#xf078;</i>

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
              class="vfs-action-btn"
              hx-target="#resource-preview-content-2"
              hx-trigger="click"
              hx-swap="innerHTML"
              hx-get="/api/v1/verified/preview?rid=${resource.rid}&resourcename=${resource.name}&volume=${resource.vname}"
              hx-headers='{"Range": "bytes=${getPreviewWindow(0)}"}'
            >Preview</button>
              
            <button 
              class="r-btn-edit"
              hx-get="/api/v1/verified/edit-form?resourcename=${resource.name}&owner=${resource.owner || 0}&group=${resource.group || 0}&perms=${resource.perms}&rid=${resource.rid}&volume=${resource.vname}"
              hx-swap="innerHTML"
              hx-trigger="click"
              hx-target="#edit-modal-2"
              hx-on::after-request="show(this.parentNode.querySelector('#edit-modal-2'))"
              >
              Edit
            </button>
            <div id="edit-modal-2" class="modal hidden darkened"></div>

            <button 
              class="r-btn-delete vfs-action-btn"
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
              onclick="document.querySelector('#selected-resource-display').classList.add('hidden');"
            >
            Close
            </button>
          </div>
        </div>
      </div>
      <div id="selected-resource-draggable-bar" class="draggable-bar"></div>
    </div>
    <hr>
    <div class="resource-details-main">
      <div class="resource-details-inner">
        <p><strong>Rid:</strong> ${resource.rid}</p>
        <p><strong>Name:</strong> ${resource.name}</p>
        <p><strong>Volume:</strong> ${resource.vname}</p>
        <p><strong>Type:</strong> ${resource.type}</p>
        <p><strong>Size:</strong> ${resource.size}</p>
        <p><strong>Permissions:</strong> ${resource.perms}</p>
        <p><strong>Created At:</strong> ${resource.createdAt}</p>
        <p><strong>Updated At:</strong> ${resource.updated_at}</p>
        <p><strong>Accessed At:</strong> ${resource.accessed_at}</p>
        <p><strong>Owner:</strong> ${resource.uid || 0}</p>
        <p><strong>Group:</strong> ${resource.gid || 0}</p>
        <p><strong>Vid:</strong> ${resource.vid || 0}</p>
      </div>
      <div id="resource-preview" class="resource-preview-window">
        
        <div class="resource-preview-main blurred">
          <div id="resource-preview-content-2" class="resource-preview-content"></div>
          <div id="resource-preview-controls">
            <div 
              id="next-arrow-left" 
              class="next-arrow vfs-action-btn"
              hx-target="#resource-preview-content-2"
              hx-trigger="click"
              hx-swap="innerHTML"
              hx-get="/api/v1/verified/preview?rid=${resource.rid}&resourcename=${resource.name}&volume=${resource.vname}"
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
              class="next-arrow vfs-action-btn"
              hx-target="#resource-preview-content-2"
              hx-trigger="click"
              hx-swap="innerHTML"
              hx-get="/api/v1/verified/preview?rid=${resource.rid}&resourcename=${resource.name}&volume=${resource.vname}"
              hx-headers='js:{"Range": "bytes=" + getPreviewWindow(+1)}'            >
              <svg width="24" height="8" viewBox="0 0 16 8" fill="none" xmlns="http://www.w3.org/2000/svg" class="arrow-icon">
                <path d="M15 4H4V1" stroke="white"/>
                <path d="M14.5 4H3.5H0" stroke="white"/>
                <path d="M15.8536 4.35355C16.0488 4.15829 16.0488 3.84171 15.8536 3.64645L12.6716 0.464466C12.4763 0.269204 12.1597 0.269204 11.9645 0.464466C11.7692 0.659728 11.7692 0.976311 11.9645 1.17157L14.7929 4L11.9645 6.82843C11.7692 7.02369 11.7692 7.34027 11.9645 7.53553C12.1597 7.7308 12.4763 7.7308 12.6716 7.53553L15.8536 4.35355ZM15 4.5L15.5 4.5L15.5 3.5L15 3.5L15 4.5Z" fill="white"/>
              </svg>
            </div>
          </div>
        </div>
      </div>

    </div>
    <hr>
    <div class="resource-details-footer">
      <div class="feedback"></div>
      <div class="r-loader hidden"><div></div></div>
    </div>
    </div>
  `;
  const btns = targetDiv.querySelectorAll(".r-btn-download, .r-btn-edit, .r-btn-delete, #preview-resource-btn, #next-arrow-right, #next-arrow-left");
  btns.forEach(button => {
    htmx.process(button);
  });

  addDragFunctionality(targetDiv);

}


// ===== DRAGGING =====
function addDragFunctionality(targetDiv) {

  const terminalHeader = targetDiv.querySelector('.draggable-bar');

  let offsetX = 0;
  let offsetY = 0;
  let isDragging = false;
  terminalHeader.addEventListener('mousedown', (e) => {
    // Calculate the distance between the mouse pointer and the container's top-left corner
    offsetX = e.clientX - targetDiv.offsetLeft;
    offsetY = e.clientY - targetDiv.offsetTop;
    isDragging = true;

    // Add global listeners so dragging works even if the mouse leaves the header
    document.addEventListener('mousemove', onMouseMove);
    document.addEventListener('mouseup', onMouseUp);
  });

  function onMouseMove(e) {
    if (!isDragging) return;
    e.preventDefault();
    // Move the container so it follows the mouse pointer
    targetDiv.style.left = (e.clientX - offsetX) + 'px';
    targetDiv.style.top  = (e.clientY - offsetY) + 'px';
  }

  function onMouseUp(e) {
    isDragging = false;
    document.removeEventListener('mousemove', onMouseMove);
    document.removeEventListener('mouseup', onMouseUp);
  }
}