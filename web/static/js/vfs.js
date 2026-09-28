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
  container.textContent = "";

  const node = getNodeAtPath(pathParts);
  if (!node) return;

  // where we are
  const crumb = document.createElement("div");
  crumb.className = "vfs-path";
  crumb.textContent = "/" + pathParts.join("/");
  container.appendChild(crumb);

  if (pathParts.length > 0) {
    const back = document.createElement("button");
    back.type = "button";
    back.textContent = "..";
    back.classList.add("back");
    back.addEventListener("click", () => {
      currentPath.pop();
      renderVFS(currentPath, container);
    });
    container.appendChild(back);
  }

  const keys = Object.keys(node).sort();
  if (!keys.length) {
    const empty = document.createElement("p");
    empty.className = "k-dim vfs-empty";
    empty.textContent = "No files yet. Upload some below.";
    container.appendChild(empty);
    return;
  }

  keys.forEach(key => {
    const entry = node[key];
    const isFile = entry.__isFile;
    const item = document.createElement("button");
    item.type = "button";
    item.textContent = (isFile || key == ".") ? key : key + "/";
    item.classList.add(isFile ? "file" : "directory");
    item.addEventListener("click", () => {
      container.querySelectorAll(".is-selected").forEach((el) => el.classList.remove("is-selected"));
      if (isFile) {
        item.classList.add("is-selected");
        displaySelectedResource([...currentPath, key].join("/"));
      } else if (key != ".") {
        currentPath.push(key);
        renderVFS(currentPath, container);
      }
    });
    container.appendChild(item);
  });
}

function displaySelectedResource(resourcePath) {
  const targetDiv = document.getElementById("selected-resource-display");

  // the tree is built as <volume>/<name parts>: match both
  let resource = null;
  for (const r of cachedResources) {
    const full = (r.vname || "default") + "/" + String(r.name || "").split("/").filter(Boolean).join("/");
    if (full === resourcePath) { resource = r; break; }
  }
  if (!resource) {
    for (const r of cachedResources) {
      if (resourcePath.includes(r.name)) { resource = r; break; }
    }
  }
  if (!resource) return;
  targetDiv.classList.remove("hidden");

  const r = {
    id: resource.rid, name: resource.name, vname: resource.vname,
    owner: resource.uid, group: resource.gid, perms: resource.perms,
  };
  const size = typeof kFmtBytes === "function" ? kFmtBytes(resource.size) : resource.size;
  const when = (v) => (typeof kFmtWhen === "function" ? kFmtWhen(v) : v);
  targetDiv.innerHTML = resourceDetailsHTML(r, {
    previewId: "resource-preview-content-2",
    vfs: true,
    draggable: true,
    closeAttr: 'data-hide="#selected-resource-display"',
    facts: [
      ["RID", resource.rid], ["Type", resource.type], ["Size", size], ["Permissions", resource.perms],
      ["Created", when(resource.createdAt)], ["Updated", when(resource.updatedAt)], ["Accessed", when(resource.accessedAt)],
      ["Owner", resource.uid || 0], ["Group", resource.gid || 0], ["VID", resource.vid || 0],
    ],
  });
  targetDiv.querySelectorAll(".r-btn-download, .r-btn-edit, .r-btn-delete, #preview-resource-btn, #next-arrow-right, #next-arrow-left").forEach(button => {
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