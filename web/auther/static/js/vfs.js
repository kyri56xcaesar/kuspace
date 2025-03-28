// Build the VFS tree from paths
function buildTree(resources) {
  // console.log(paths);
  const root = {};
  resources.forEach(resource => {
    const parts = resource.name.split("/").filter(Boolean);
    // console.log(parts);
    let node = root;
    parts.forEach((part, index) => {
      if (!node[part]) {
        node[part] = {
          __isFile: (index === parts.length - 1) && (parts[parts.length - 1] != "."),
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
        console.log("File selected:", [...currentPath, key].join("/"));
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

  let resourceTarget;

  for (resource of resources) {
    if ("/"+resourcePath=== resource.name) {
      resourceTarget = resource;
      break;
    }
  }

  console.log(resourceTarget);

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
              onclick="downloadResource('${resourceTarget.name}')"
            >
              Download
            </button>
              
            <button 
              class="r-btn-edit"
              hx-get="/api/v1/verified/edit-form?resourcename=${resourceTarget.name}&owner=${resourceTarget.uid}&group=${resourceTarget.gid}&perms=${resourceTarget.perms}&rid=${resourceTarget.rid}"
              hx-swap="innerHTML"
              hx-trigger="click"
              hx-target="#edit-modal"
              hx-on::after-request="show(document.getElementById('edit-modal'))"
              >
              Edit
            </button>
            <button 
              class="r-btn-delete"
              hx-delete="/api/v1/verified/rm?rids=${resourceTarget.rid}"
              hx-trigger="click"
              hx-swap="none"
              hx-confirm="Are you sure you want to delete resource ${resourceTarget.name}?"

              hx-on::before-request="show(document.querySelector('.r-loader'))"
            >
              Delete
            </button>

            <button
              id="close-r-selected-display"
              onclick="console.log(this);document.querySelector('#selected-resource-display').classList.add('hidden');"
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
        <p><strong>Rid:</strong> ${resourceTarget.rid}</p>
        <p><strong>Name:</strong> ${resourceTarget.name}</p>
        <p><strong>Type:</strong> ${resourceTarget.type}</p>
        <p><strong>Size:</strong> ${resourceTarget.size}</p>
        <p><strong>Permissions:</strong> ${resourceTarget.perms}</p>
        <p><strong>Created At:</strong> ${resourceTarget.created_at}</p>
        <p><strong>Updated At:</strong> ${resourceTarget.updated_at}</p>
        <p><strong>Accessed At:</strong> ${resourceTarget.accessed_at}</p>
        <p><strong>Owner:</strong> ${resourceTarget.uid}</p>
        <p><strong>Group:</strong> ${resourceTarget.gid}</p>
        <p><strong>Volume:</strong> ${resourceTarget.vid}</p>
      </div>
      <div id="resource-preview" class="resource-preview-window">
        <button
          id="preview-resource-btn"
          hx-target="#resource-preview-content"
          hx-trigger="click"
          hx-swap="innerHTML"
          hx-get="/api/v1/verified/preview?rid=${resourceTarget.rid}&resourcename=${resourceTarget.name}"
          hx-headers='{"Range": "bytes=0-4095"}'
        >Preview</button>
        <div class="resource-preview-main blurred">
          <div id="resource-preview-content"></div>
          <div id="resource-preview-controls">
            <div id="next-arrow-left" class="next-arrow">
              <svg width="24" height="8" viewBox="0 0 16 8" fill="none" xmlns="http://www.w3.org/2000/svg" class="arrow-icon">
              <g transform="scale(-1,1) translate(-16,0)">
                <path d="M15 4H4V1" stroke="white"/>
                <path d="M14.5 4H3.5H0" stroke="white"/>
                <path d="M15.8536 4.35355C16.0488 4.15829 16.0488 3.84171 15.8536 3.64645L12.6716 0.464466C12.4763 0.269204 12.1597 0.269204 11.9645 0.464466C11.7692 0.659728 11.7692 0.976311 11.9645 1.17157L14.7929 4L11.9645 6.82843C11.7692 7.02369 11.7692 7.34027 11.9645 7.53553C12.1597 7.7308 12.4763 7.7308 12.6716 7.53553L15.8536 4.35355ZM15 4.5L15.5 4.5L15.5 3.5L15 3.5L15 4.5Z" fill="white"/>
              </g>
              </svg>
            </div>
            <div>
              <span id="page-index">1</span>
            </div>
            <div id="next-arrow-right" class="next-arrow">
              <svg width="24" height="8" viewBox="0 0 16 8" fill="none" xmlns="http://www.w3.org/2000/svg" class="arrow-icon">
                <path d="M15 4H4V1" stroke="white"/>
                <path d="M14.5 4H3.5H0" stroke="white"/>
                <path d="M15.8536 4.35355C16.0488 4.15829 16.0488 3.84171 15.8536 3.64645L12.6716 0.464466C12.4763 0.269204 12.1597 0.269204 11.9645 0.464466C11.7692 0.659728 11.7692 0.976311 11.9645 1.17157L14.7929 4L11.9645 6.82843C11.7692 7.02369 11.7692 7.34027 11.9645 7.53553C12.1597 7.7308 12.4763 7.7308 12.6716 7.53553L15.8536 4.35355ZM15 4.5L15.5 4.5L15.5 3.5L15 3.5L15 4.5Z" fill="white"/>
              </svg>
            </div>
          </div>
        </div>
      </div>

      <div id="edit-modal" class="modal hidden darkened"></div>
    </div>
    <hr>
    <div class="resource-details-footer">
      <div class="feedback"></div>
      <div class="r-loader hidden"><div></div></div>
    </div>
    </div>
  `;
  const btns = targetDiv.querySelectorAll(".r-btn-download, .r-btn-edit, .r-btn-delete, #preview-resource-btn");
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