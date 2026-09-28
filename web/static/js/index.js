


let domReady = (cb) => {
  document.readyState === 'interactive' || document.readyState === 'complete'
    ? cb()
    : document.addEventListener('DOMContentLoaded', cb)
};
domReady(() => {
  document.body.style.visibility = 'visible';

  // dark mode memory: applied right away so the page never flashes light
  const tglBtn = document.getElementById("dark-mode-toggle");
  if (tglBtn) {
    tglBtn.checked = isDarkMode();
  }
  applyTheme();

  // TIPS/INFO BUTTONS
  const toggleButton = document.querySelectorAll(".toggle-button-collapse");
  toggleButton.forEach(toggleButton => {
    toggleButton.addEventListener("click", () => {
      toggleButton.classList.toggle("collapsed");
      // get the closest h1 or p or span...
      const target = toggleButton.closest(".info").querySelector(".target");
      target.classList.toggle("collapsed");  
      toggleButton.setAttribute("aria-expanded", target.classList.contains("collapsed") ? "false" : "true");
    });
  });

  // logout
  const logout_btn = document.getElementById("logout-a");
  if (logout_btn) {
    logout_btn.addEventListener("click", () => {
      console.log("logging out");
      // window.location.href = "/api/v1/login";
    });
  }

  const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
  sleep(1000).then(() => {
    // tips memory
    const tipsBtn = document.getElementById("tips-mode-toggle");
    if (tipsBtn) {
      const collapsibleElmnts = document.querySelectorAll(".collapsible, .fading");
      if(localStorage.getItem("tipsMode") === "true") {
        collapsibleElmnts.forEach((el) => {
          if (el.tagName === "BUTTON") {
            el.click();
          } else {
            el.classList.add("collapsed");
          }    
          el.classList.add("hidden");
        });
        tipsBtn.checked = true;
      } else {
        collapsibleElmnts.forEach((el) => {
          if (el.tagName === "BUTTON") {
            el.click();
          } else {
            el.classList.remove("collapsed");
          }    
          el.classList.remove("hidden");
        });
        tipsBtn.checked = false;
      }
    }

  });

});

// Dark mode has one source of truth: the "darkMode" key in localStorage.
// applyTheme() stamps it on <html> (theme-dark) for scoped CSS, and on every
// .darkened element (plus the few fragments styled via .dark-mode) - including
// content HTMX swaps in later, see the htmx:afterSettle handler.
const THEMED = ".darkened, #all-users-table, #all-groups-table, .job-display-entry, .app-card";

function isDarkMode() {
  try {
    return localStorage.getItem("darkMode") === "true";
  } catch (e) {
    return false;
  }
}

function applyTheme(root = document) {
  const dark = isDarkMode();
  document.documentElement.classList.toggle("theme-dark", dark);
  if (root !== document && root.matches && root.matches(THEMED)) {
    root.classList.toggle("dark-mode", dark);
  }
  root.querySelectorAll(THEMED).forEach((el) => el.classList.toggle("dark-mode", dark));
}

function toggleDarkMode() {
  const tglBtn = document.getElementById("dark-mode-toggle");
  const dark = tglBtn ? tglBtn.checked : !isDarkMode();
  try {
    localStorage.setItem("darkMode", String(dark));
  } catch (e) {
    // storage blocked: theme still switches for this page view
    document.documentElement.classList.toggle("theme-dark", dark);
  }
  applyTheme();
}

function toggleCollapses() {
  const collapsibleElmnts = document.querySelectorAll(".collapsible, .fading");
  collapsibleElmnts.forEach((el) => {
    if (el.tagName === "BUTTON") {
      el.click();
    } else {
      el.classList.toggle("collapsed");
    }
    el.classList.toggle("hidden");
  });
  const tipsBtn = document.getElementById("tips-mode-toggle");
  if (tipsBtn) {
    localStorage.setItem("tipsMode", tipsBtn.checked);
  }

}

function showSection(sectionId) {
  const sections = document.querySelectorAll('.content-section');
  if (sections) {
    sections.forEach(section => {
      if (section.id === sectionId) {
        section.classList.remove('hidden');
      } else {
        section.classList.add('hidden');
      }
    });
  }
}

function showSubSection(divId, sectionId) {
  const parentDiv = document.getElementById(divId);
  if (!parentDiv) {
    console.error(`Parent div with ID ${divId} not found.`);
    return;
  }
  const subsections = parentDiv.querySelectorAll('.subsection');
  if (subsections) {
    subsections.forEach(subsection => {
      if (subsection.id === sectionId) {
        subsection.classList.remove('hidden');
      } else {
        subsection.classList.add('hidden');
      }
    })
  }
}

/* removes the hidden class */
function show(container) {
  if (container){
    container.classList.remove('hidden');
  }
}
/* add the hidden class to the container, which makes the display to none*/
function hide(container) {
  if (container) {
    container.classList.add('hidden');
  } 
}

function toggleHidden(targetId, className) {
  let targetDiv = document.querySelector(targetId);
  document.querySelectorAll(className).forEach((element) => {
    if (element.id === targetDiv.id) { 
      element.classList.remove('hidden');
    } else {
      element.classList.add('hidden');
    }
  });
}

function copyToClipboard(selector, copyBtnId) {
  const element = document.querySelector(selector);
  if (element) {
    const text = element.textContent.trim(); // Trim any extra spaces
    navigator.clipboard.writeText(text).then(() => {
      const copyBtn = document.getElementById(copyBtnId);
      if (copyBtn && !copyBtn.classList.contains("is-copied")) {
        const label = copyBtn.innerHTML;
        copyBtn.classList.add("is-copied");
        copyBtn.innerHTML = '<i class="fa-solid fa-check" aria-hidden="true"></i> Copied';
        setTimeout(() => {
          copyBtn.innerHTML = label;
          copyBtn.classList.remove("is-copied");
        }, 2000);
      }
    }).catch(err => {
      alert("Failed to copy: " + err);
    });
  }
}

function getCookie(name) {
  const value = `; ${document.cookie}`;
  const parts = value.split(`; ${name}=`);
  if (parts.length === 2) return parts.pop().split(';').shift();
  return '';
}

function downloadResource(href) {
  const link = document.createElement("a");
  link.href = href;
  link.download = ""; // tells browser to treat it as a download
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
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

var previewBytes = 4095;
var previewIndex = 0;
function getPreviewWindow(inc) {
  // direction must be either +1 or -1
  if (inc < -1 || inc > 1) {
    return "0-" + previewBytes.toString();
  }

  // reset
  if (inc === 0) {
    previewIndex = 0;
  }

  previewIndex += inc;
  if (previewIndex < 0) {
    previewIndex = 0;
  }

  let start = previewIndex * previewBytes;
  if (start < 0) {
    start = 0;
  }

  let end = (previewIndex + 1) * previewBytes;

  
  // update page index;
  let indexDisplay = document.getElementById("page-index");
  if (indexDisplay) {
    indexDisplay.textContent = previewIndex.toString();
  }


  return start.toString() + "-" + end.toString();

}

// htmx events handling
document.addEventListener('htmx:afterSettle', function(event) {
  const triggeringElement = event.detail.elt;
  const triggeringElementId = triggeringElement.id;

  // whatever was swapped in follows the current theme
  applyTheme(event.detail.target);

  if (triggeringElementId === 'fetch-applications-display') {
    const appSelector = document.getElementById('language-selector');
    const current = appSelector.value || "duckdb";
    appSelector.innerHTML = '';
    event.detail.target.querySelectorAll('.app-card').forEach((item) => {
      const option = document.createElement('option');
      const nameEl = item.querySelector('.app-name');
      const appName = (nameEl.dataset.v || nameEl.textContent.trim().split(" ")[0]).trim();
      option.value = appName;
      option.textContent = appName;
      option.selected = appName === current;
      appSelector.appendChild(option);
    });
  }
})

document.addEventListener('htmx:beforeSwap', function(event) {
  const triggeringElement = event.detail.elt;
  const triggeringElementId = triggeringElement.id;

  
  if (triggeringElementId === "gshell-spawner") {
    let newShell = newTerminal();
    if (newShell) {
      event.detail.target = newShell;
    } else {
      event.preventDefault();
    }
  }
});

document.addEventListener('htmx:afterSwap', function (event) {
  const triggeringElement = event.detail.elt;
  const triggeringElementId = triggeringElement.id;

  // console.log(triggeringElementId);

  if (triggeringElementId === 'hasher-verify-btn') {
    const verifyResultElement = document.getElementById('verify-result');
    if (verifyResultElement) {
      const result = verifyResultElement.textContent.trim();
      if (result.toLowerCase() === 'true') {
        verifyResultElement.className = 'true';
      } else if (result.toLowerCase() === 'false') {
        verifyResultElement.className = 'false';
      }
    }
  } else if (triggeringElementId.startsWith('gshell-container')) {
    // Grab that specific shell and give it the terminal features
    giveFunctionality(triggeringElement); 
  } else if (triggeringElementId === "fetch-jobs-div") {
    const cacheResultsLi = document.getElementById("fetch-jobs-div").querySelectorAll("li");
    setupSearchBar(document.querySelector("#existing-jobs-container .search-bar"), cacheResultsLi);
    // console.log(cacheJobResultsLi);
  } else if (triggeringElementId === "fetch-jobs-div-2") {
    const cacheResultsLi = document.getElementById("fetch-jobs-div-2").querySelectorAll("li");
    setupSearchBar(document.querySelector("#existing-jobs-container-2 .search-bar"), cacheResultsLi);
  } else if (triggeringElementId === "volumes-target") {
    cacheVolumeResults = document.getElementById("volumes-target").querySelectorAll(".v-body");
    // console.log(cacheVolumeResults);
    // vol upload file

    cacheVolumeResults.forEach((volume) => {
      let uploadBtn = volume.querySelector("#uploadButton");
      let finp = volume.querySelector("#fileInput");
  
      uploadBtn.addEventListener("click", () => {
        finp.click(); // open the file dialog
      });
      
      finp.addEventListener("change", async () => {
        const vname = volume.querySelector(".name").innerText;
        const files = finp.files;

        if (!files.length) return;

        const formData = new FormData();
        for (let file of files) {
          formData.append("files", file); // keep 'files' plural to match backend
        }
      
        try {
          const response = await fetch("/api/v1/verified/upload", {
            method: "POST",
            headers: {
              "X-Volume-Target": vname 
            },
            body: formData
          });
        
          const result = await response.text(); // or response.json()
          const feedback = volume.querySelector(".feedback");
          feedback.textContent = result;
        } catch (err) {
          console.error("Upload error:", err);
          const feedback = volume.querySelector(".feedback");
          feedback.textContent = "Upload failed: " + err.message;
        }
      });

    });


  } else if (triggeringElementId === "resources-main") {
    let parent = document.querySelector("#resource-list-table > tbody");
    if (parent) {
      cacheResourceResults = parent.querySelectorAll("tr");
      // console.log(cacheResourceResults);
    }
    // populte eventListeners and edit logic
    addResourceListListeners()
  } else if (triggeringElementId === "fetch-groups-results") {
    const cacheGroupResults = triggeringElement.querySelector("tbody").querySelectorAll("tr");
    // Groups search
    const gSearch = triggeringElement.querySelector("#group-search");
    let gSearchBy = "name";
    const gSearchSelector = triggeringElement.querySelector(".k-view-table-header").querySelector(".k-search-by");
    gSearchSelector.value = gSearchBy;
    gSearchSelector.addEventListener("input", (event) => {
      gSearchBy = gSearchSelector.value;
      gSearch.placeholder = "Search by '" + gSearchBy+"'";
    });

    gSearch.value = "";
    gSearch.addEventListener("input", function() {
      if (cacheGroupResults.length === 0) {// empty cache, must fetch 

      }
      const searchValue = gSearch.value;
      // console.log("searching by " + searchBy + " at " + searchValue);
      // do search and display
      cacheGroupResults.forEach((li) => {
        switch (gSearchBy) {
          case "name":
            if (!li.querySelector(".name").innerText.includes(searchValue)) {
              li.classList.add("hidden");
            } else {
              li.classList.remove("hidden");
            }
            break;
          case "gid":
            if (!li.querySelector(".gid").innerText.includes(searchValue)) {
              li.classList.add("hidden");
            } else {
              li.classList.remove("hidden");
            }
            break;
          case "users":
            if (!li.querySelector(".users").textContent.includes(searchValue)) {
              li.classList.add("hidden");
            } else {
              li.classList.remove("hidden");
            }
            break;
        }
      });
    
    });

  } else if (triggeringElementId === "fetch-users-results") {
    const cacheUserResults = triggeringElement.querySelector("tbody").querySelectorAll("tr");
    // console.log(cacheUserResults);
    // Users search 
    let uSearchBy = "name";
    const uSearchSelector = triggeringElement.querySelector(".k-view-table-header").querySelector(".k-search-by");
    const uSearch = triggeringElement.querySelector("#user-search");
    uSearchSelector.value = uSearchBy;
    uSearchSelector.addEventListener("input", (event) => {
      uSearchBy = uSearchSelector.value;
      uSearch.placeholder = "Search by '" + uSearchBy+"'";
    });

    uSearch.value = "";
    uSearch.addEventListener("input", function() {
      if (cacheUserResults.length === 0) {// empty cache, must fetch 

      }
      const searchValue = uSearch.value;
      // console.log("searching by " + searchBy + " at " + searchValue);
      // do search and display
      cacheUserResults.forEach((li) => {
        switch (uSearchBy) {
          case "name":
            if (!li.querySelector(".name").innerText.includes(searchValue)) {
              li.classList.add("hidden");
            } else {
              li.classList.remove("hidden");
            }
            break;
          case "groups":
            if (!li.querySelector(".groups").textContent.includes(searchValue)) {
              li.classList.add("hidden");
            } else {
              li.classList.remove("hidden");
            }
            break;
          case "email":
            if (!li.querySelector(".email").innerText.includes(searchValue)) {
              li.classList.add("hidden");
            } else {
              li.classList.remove("hidden");
            }
            break;
          case "home":
            if (!li.querySelector(".home").innerText.includes(searchValue)) {
              li.classList.add("hidden");
            } else {
              li.classList.remove("hidden");
            }
            break;
          case "uid":
            if (!li.querySelector(".uid").innerText.includes(searchValue)) {
              li.classList.add("hidden");
            } else {
              li.classList.remove("hidden");
            }
            break;
        }
      });
    
    });
  }

});

document.addEventListener('htmx:beforeRequest', function(event) {
  const triggeringElement = event.detail.elt;

  // handle different cases: 
  if (triggeringElement.id === 'inp-text' && triggeringElement.value === '') {
    event.preventDefault();
    document.getElementById("generated-hash").innerText = '';
  }
});

document.addEventListener('htmx:afterRequest', function (event) {
  const triggeringElement = event.detail.elt;

  // console.log('triggering element: ', triggeringElement);
  // console.log('triggering element id: ', triggeringElement.id);
  // console.log('triggering element class: ', triggeringElement.className);

  // Handle different cases
  // all users fetch
  if (triggeringElement.id === 'fetch-users-results') { 
    if (event.detail.xhr.status >= 300 && event.detail.xhr.status < 400) {
      const redirectLocation = event.detail.xhr.getResponseHeader("Location");
      if (redirectLocation) {
        window.location.href = redirectLocation;
      } else if (event.detail.xhr.status >= 200 && event.detail.xhr.status < 300) {
        // done, no redirect to follow
      } else {
        console.error("Redirect location not found in the response."); 
      }  
    }
    // reload users fetch
  
  } else if (triggeringElement.id === 'fetch-resources-form') {
    if (event.detail.xhr.status === 404) {
      const rtbl = document.getElementById("resource-list-table");
      if (rtbl) {
        rtbl.innerHTML = "";
      }
      const feedback = document.getElementById("resources-feedback");
      feedback.classList.remove('hidden');
      feedback.textContent = "No resources found.";
      feedback.classList.add('red');
      setTimeout(() => {
        feedback.classList.add('hidden');
        feedback.textContent = '';
      }, 4000);
    }
  } else if (triggeringElement.id === 'add-user-form') {
    // new user creation (from admin)
    const feedback = triggeringElement.parentNode?.querySelector('.feedback');
    triggeringElement.reset();

    if (event.detail.xhr.status >= 200 && event.detail.xhr.status < 300) {
      document.getElementById('reload-btn').dispatchEvent(new Event('click'));
      triggeringElement.classList.add('check-highlight');
      feedback.textContent = event.detail.xhr.responseText.replace(/[{}]/g, '');
      setTimeout(()=>{
        triggeringElement.classList.remove('check-highlight');
        feedback.textContent = '';
      }, 4000);

    } else if (event.detail.xhr.status < 400) {
      // 3xx: nothing to show
    } else if (event.detail.xhr.status < 500) {
      triggeringElement.classList.add('error-highlight');
      feedback.textContent = event.detail.xhr.responseText.replace(/[{}]/g, '');
      setTimeout(()=>{
        triggeringElement.classList.remove('error-highlight');
        feedback.textContent = '';
      }, 4000);
    }
  // deleting users by admin 
  
  
  } else if (typeof triggeringElement.id === "string" && triggeringElement.id.startsWith('delete-btn-')) {
    if (event.detail.xhr.status >= 200 && event.detail.xhr.status < 300) {
      // Successfully deleted
      const rowId = triggeringElement.closest('tr').id; // Get the table row ID
      document.getElementById(rowId).remove(); // Remove the table row
      document.getElementById('reload-btn').dispatchEvent(new Event('click')); 
    } else {
      // Failed delete, apply red border
      const rowId = triggeringElement.closest('tr').id;
      document.getElementById(rowId).classList.add('error-highlight');
    }
  
  
  } else if (typeof triggeringElement.id === "string" && triggeringElement.id.startsWith('delete-grp-btn')) {
    if (event.detail.xhr.status >= 200 && event.detail.xhr.status < 300) {
      // Successfully deleted
      const rowId = triggeringElement.closest('tr').id; // Get the table row ID
      document.getElementById(rowId).remove(); // Remove the table row
      document.getElementById('reload-groups-btn').dispatchEvent(new Event('click')); 
    } else {
      // Failed delete, apply red border
      const rowId = triggeringElement.closest('tr').id;
      document.getElementById(rowId).classList.add('error-highlight');
    }
  // logging out (generic)
 
  
  } else if (typeof triggeringElement.id === "string" && triggeringElement.id.startsWith("logout")) {
    if (event.detail.xhr.status >= 200 && event.detail.xhr.status < 400) {
      window.location.href = "/api/v1/login";
    }
  // hasher related
 

  } else if (typeof triggeringElement.id === "string" && triggeringElement.id.startsWith("inp-text")) {
    if (event.detail.xhr.status >= 400) {
      document.getElementById("generated-hash").textContent = "";
    }  


  // submit user patch by admin
  } else if (typeof triggeringElement.id === "string" && triggeringElement.id.startsWith("submit-btn-")) {
    if (event.detail.xhr.status >= 200 && event.detail.xhr.status < 300) {
      const row = triggeringElement.closest('tr');
      if (row) {
        row.classList.add('check-highlight');
        setTimeout(()=>{
          row.classList.remove('check-highlight')
        }, 2000);
      }
      document.getElementById('reload-btn').dispatchEvent(new Event('click'));
    } else if (event.detail.xhr.status >= 500 || event.detail.xhr.status === 400){
      const row = triggeringElement.closest('tr');
      if (row) {
        row.classList.add('error-highlight');
        setTimeout(()=>{
          row.classList.remove('error-highlight');
        }, 2000);
      }
    } else if (event.detail.xhr.status === 404) {
        const row = triggeringElement.closest('tr');
        if (row) {
          row.classList.add('warning-highlight');
          setTimeout(()=>{
            row.classList.remove('warning-highlight')
          }, 2000);
        }
    }
 

  } else if (triggeringElement.id === 'add-group-form') {
    const feedback = triggeringElement.previousElementSibling?.querySelector('.feedback');
    triggeringElement.reset();
    if (event.detail.xhr.status >= 200 && event.detail.xhr.status <300) {
      document.getElementById('reload-groups-btn').dispatchEvent(new Event('click'));
      triggeringElement.classList.add('check-highlight');
      feedback.textContent = event.detail.xhr.responseText.replace(/[{}]/g, '');
      setTimeout(()=>{
        triggeringElement.classList.remove('check-highlight');
        feedback.textContent = '';
      }, 4000);
    }  else if (event.detail.xhr.status >= 400 && event.detail.xhr.status < 500) {
      triggeringElement.classList.add('warning-highlight');
      feedback.textContent = event.detail.xhr.responseText.replace(/[{}]/g, '');
      setTimeout(()=>{
        triggeringElement.classList.remove('warning-highlight');
        feedback.textContent = '';
      }, 4000);
    } else if (event.detail.xhr.status >= 500) {
      triggeringElement.classList.add('error-highlight');
      feedback.textContent = event.detail.xhr.responseText.replace(/[{}]/g, '');
      setTimeout(()=>{
        triggeringElement.classList.remove('error-highlight');
        feedback.textContent = '';
      }, 4000);
    }
    

  
  } else if (triggeringElement.id === 'upload-files-form') {
      setTimeout(() => {
        hide(document.getElementById('progress-container'))
      }, 2000);


      if (event.detail.xhr.status >= 200 && event.detail.xhr.status < 300 && fileUploadModule) {
        fileUploadModule.reset();

        kUploadState("is-success", "File(s) uploaded", 10000);
        // reload resources
        document.querySelector("#fetch-resources-form").requestSubmit();
        document.getElementById("fetch-resources-form").scrollTo({ top: 0, behavior: "smooth"});

      } else if (event.detail.xhr.status >= 300) {
        kUploadState("is-error", "Failed to upload.", 4000);
      }
  } else if (triggeringElement.id === 'upload-files-form-dash') {
      setTimeout(() => {
        hide(document.getElementById('progress-container'))
      }, 2000);


      if (event.detail.xhr.status >= 200 && event.detail.xhr.status < 300 && fileUploadModule) {
        fileUploadModule.reset();

        kUploadState("is-success", "File(s) uploaded", 10000);

      } else if (event.detail.xhr.status >= 300) {
        kUploadState("is-error", "Failed to upload.", 4000);
      }
  } else if (triggeringElement.classList.contains("r-btn-delete") && !triggeringElement.classList.contains("vfs-action-btn")) {
    const feedback = document.getElementById("resources-feedback");
    feedback.classList.remove('hidden');
    feedback.textContent = event.detail.xhr.responseText.replace(/[{}]/g, '');
    if (event.detail.xhr.status >= 200 && event.detail.xhr.status < 300) {
      document.querySelector("#fetch-resources-form").requestSubmit();
      document.getElementById("fetch-resources-form").scrollTo({ top: 0, behavior: "smooth"});
      feedback.classList.add('green');
      // remove the selected 
      document.querySelectorAll("#resource-list-table tbody tr").forEach((r) => r.classList.remove("selected"));
      document.getElementById("resource-details").innerHTML = "";
    } else {
      feedback.classList.add('red');
    }
    setTimeout(() => {
      feedback.classList.add('hidden');
      feedback.textContent = '';
    }, 8000);
  } else if (triggeringElement.id === 'root-dashboard-loader') {
      if (event.detail.xhr.status >= 200 && event.detail.xhr.status < 300) {
        const profileMenu = document.querySelector(".profile-menu");
        const profileButton = document.querySelector(".profile-button");
        profileButton.addEventListener("click", () => {
            profileMenu.classList.toggle("open");
        });
        document.addEventListener("click", (event) => {
          if (!profileMenu.contains(event.target)) {
            profileMenu.classList.remove("open");
          }
        });
      }
  } else if (triggeringElement.id === 'preview-resource-btn') {
    if (event.detail.xhr.status >= 200 && event.detail.xhr.status < 300) {
      const host = triggeringElement.closest("#resource-details, #selected-resource-display") || document;
      host.querySelector(".resource-preview-main")?.classList.remove("blurred");
    }
  } else if (triggeringElement.id === 'load-users-to-cache') {
    if (event.detail.xhr.status >= 200 && event.detail.xhr.status < 300) {
      const rawResponse = event.detail.xhr.responseText;
      try {
        cachedUsers = JSON.parse(rawResponse); 
        // console.log("Fetched users:", cachedUsers);
      } catch (error) {
        console.error("Could not parse JSON:", error, rawResponse);
      }
    }
  
  } else if (triggeringElement.id === 'load-groups-to-cache') {
    if (event.detail.xhr.status >= 200 && event.detail.xhr.status < 300) {
      const rawResponse = event.detail.xhr.responseText;
      try {
        cachedGroups = JSON.parse(rawResponse); 
        // console.log("Fetched groups:", cachedGroups);
      } catch (error) {
        console.error("Could not parse JSON:", error, rawResponse);
      }
    }
  } else if (triggeringElement.id === 'load-resources-to-cache' || triggeringElement.id === 'vfs' || triggeringElement.id === 'vfs-reload') {
    if (event.detail.xhr.status >= 200 && event.detail.xhr.status < 300) {
      const rawResponse = event.detail.xhr.responseText;
      try {
        cachedResources = JSON.parse(rawResponse); 
        // console.log("Fetched resources:", cachedResources);
        if (triggeringElement.id === 'vfs') {
          // vfs
          currentPath = [];
          vfsRoot = {};
          vfsRoot = buildTree(cachedResources);
          renderVFS(currentPath, triggeringElement, {});
        } else if (triggeringElement.id === 'vfs-reload') {
          currentPath = [];
          vfsRoot = {};
          vfsRoot = buildTree(cachedResources);
          renderVFS(currentPath, document.getElementById("vfs"), {});
        }
      } catch (error) {
        console.error("Could not parse JSON:", error, rawResponse);
      }
    }

  } else if (triggeringElement.id === 'change-password-form') {
    const feedback = document.getElementById("user-actions-feedback");
    feedback.classList.remove('hidden');
    feedback.textContent = event.detail.xhr.responseText.replace(/[{}]/g, '');
    if (event.detail.xhr.status >= 200 && event.detail.xhr.status < 300) {
      triggeringElement.reset();
      feedback.classList.add('green');
      triggeringElement.classList.add('is-ok');
    } else {
      triggeringElement.reset();
      feedback.classList.add('red');
      triggeringElement.classList.add('is-bad');
    }
    setTimeout(() =>{
      feedback.classList.add('hidden');
      feedback.textContent = '';
      feedback.classList.remove('green', 'red');
      triggeringElement.classList.remove('is-ok', 'is-bad');
    }, 8000);
  } else if (triggeringElement.id === 'email-change-form') {
    const feedback = document.getElementById("user-actions-feedback");
    feedback.classList.remove('hidden');
    feedback.textContent = event.detail.xhr.responseText.replace(/[{}]/g, '');
    if (event.detail.xhr.status >= 200 && event.detail.xhr.status < 300) {
      triggeringElement.reset();
      triggeringElement.classList.add('is-ok');
      feedback.classList.add('green');
    } else {
      triggeringElement.reset();
      triggeringElement.classList.add('is-bad');
      feedback.classList.add('red');
    }
    setTimeout(() =>{
      feedback.classList.add('hidden');
      feedback.classList.remove('green', 'red');
      feedback.textContent = '';
      triggeringElement.classList.remove('is-ok', 'is-bad');
    }, 6000);
  } else if (triggeringElement.id === 'delete-volume-btn') {
    if (event.detail.xhr.status >= 200 && event.detail.xhr.status < 300) {
      document.getElementById('fetch-volumes-btn').dispatchEvent(new Event('click'));
      const feedback = triggeringElement.parentNode.parentNode.querySelector('.feedback');
      feedback.textContent = event.detail.xhr.responseText.replace(/[{}]/g, '');
      feedback.classList.add('green');
      feedback.classList.remove('hidden');
      setTimeout(() => {
        feedback.textContent = '';
        feedback.classList.add('hidden');
      }, 4000);
    } else if (event.detail.xhr.status < 400) {
      //todo
      const feedback = triggeringElement.parentNode.parentNode.querySelector('.feedback');
      feedback.textContent = event.detail.xhr.responseText.replace(/[{}]/g, '');
      feedback.classList.add('red');
      feedback.classList.remove('hidden');
      setTimeout(() => {
        feedback.textContent = '';
        feedback.classList.add('hidden');

      }, 4000);
    } else if (event.detail.xhr.status < 500) {
      //todo
      const feedback = triggeringElement.parentNode.parentNode.querySelector('.feedback');
      feedback.textContent = event.detail.xhr.responseText.replace(/[{}]/g, '');
      feedback.classList.add('red');
      feedback.classList.remove('hidden');
      setTimeout(() => {
        feedback.textContent = '';
        feedback.classList.add('hidden');

      }, 4000);
    }
  } else if (triggeringElement.id === 'create-volume-form') {
    const feedback = document.querySelector("#create-volume-modal .feedback");
    feedback.classList.remove('hidden');
    feedback.textContent = event.detail.xhr.responseText.replace(/[{}]/g, '');
    if (event.detail.xhr.status >= 200 && event.detail.xhr.status < 300) {
      document.getElementById('create-volume-modal').classList.add("hidden");
      document.getElementById('fetch-volumes-btn').dispatchEvent(new Event('click'));
      triggeringElement.reset();
      feedback.classList.add('green');
    } else if (event.detail.xhr.status < 400) {
      //todo
      triggeringElement.reset();
      feedback.classList.add('red');
    } else if (event.detail.xhr.status < 500) {
      //todo
      triggeringElement.reset();
      feedback.classList.add('red');
    }
    setTimeout(() => {
      feedback.textContent = '';
      feedback.classList.add('hidden');
    }, 8000);
  } else if (triggeringElement.id === 'fetch-volumes-btn' || triggeringElement.id === 'fetch-volumes-display') {
    if (event.detail.xhr.status >= 200 && event.detail.xhr.status < 300) {
      const volumeList = document.getElementById('volumes-target');
      if (volumeList) {
        volumeList.scrollTo({ top: 0, behavior: "smooth"});
      }
    } else if (event.detail.xhr.status < 400) {
      //todo
    } else if (event.detail.xhr.status < 500) {
      //todo
    }
  } else if (triggeringElement.id === 'job-create-form') {
    const feedback = triggeringElement?.querySelector('.feedback');
    const button = triggeringElement.querySelector('button[type="submit"]');
    if (event.detail.xhr.status >= 200 && event.detail.xhr.status < 300) {
      // document.getElementById('reload-btn').dispatchEvent(new Event('click'));
      const feedBackDiv = document.querySelector("#job-feedback");
      feedBackDiv.classList.remove('hidden');
      createFeedbackPanel(JSON.parse(event.detail.xhr.responseText).jid, feedBackDiv.querySelector('.feedback-messages'));
      feedback.textContent = event.detail.xhr.responseText.replace(/[{}]/g, '');
      setTimeout(() => {
        button.classList.add('check-highlight');
        setTimeout(()=>{
          button.classList.remove('check-highlight');
          feedback.textContent = '';
        }, 10000);
      }, 2000);
    } else if (event.detail.xhr.status < 400) {
      // 3xx: nothing to show
    } else if (event.detail.xhr.status < 500) {
      setTimeout(() => {
        button.classList.add('error-highlight');
        feedback.textContent = event.detail.xhr.responseText.replace(/[{}]/g, '');
        setTimeout(()=>{
          button.classList.remove('error-highlight');
          feedback.textContent = '';
        }, 8000);
      }, 2000);
    }
  } else if (triggeringElement.id === 'modify-job-form') {
    const feedback = triggeringElement.parentNode.parentNode.querySelector(".feedback");
    feedback.classList.remove('hidden');
    feedback.textContent = event.detail.xhr.responseText.replace(/[{}]/g, '');
    if (event.detail.xhr.status >= 200 && event.detail.xhr.status < 300) {
      triggeringElement.parentNode.parentNode.classList.add("hidden");
      triggeringElement.reset();
      feedback.classList.add('green');
    } else if (event.detail.xhr.status < 400) {
      //todo
      triggeringElement.reset();
      feedback.classList.add('red');
    } else if (event.detail.xhr.status < 500) {
      //todo
      triggeringElement.reset();
      feedback.classList.add('red');
    }
    setTimeout(() => {
      feedback.textContent = '';
      feedback.classList.add('hidden');
    }, 8000);
  } else if (triggeringElement.className === 'modify-app') {
    const feedback = triggeringElement.parentNode.parentNode.querySelector(".feedback");
    feedback.classList.remove('hidden');
    feedback.textContent = event.detail.xhr.responseText.replace(/[{}]/g, '');
    if (event.detail.xhr.status >= 200 && event.detail.xhr.status < 300) {
      triggeringElement.parentNode.parentNode.classList.add("hidden");
      triggeringElement.reset();
      feedback.classList.add('green');
    } else if (event.detail.xhr.status < 400) {
      //todo
      triggeringElement.reset();
      feedback.classList.add('red');
    } else if (event.detail.xhr.status < 500) {
      //todo
      triggeringElement.reset();
      feedback.classList.add('red');
    }
    setTimeout(() => {
      feedback.textContent = '';
      feedback.classList.add('hidden');
    }, 8000);
  } 

  if (triggeringElement.classList.contains('vfs-action-btn')) {
    if (event.detail.xhr.status >= 200 && event.detail.xhr.status < 300) {
      if (triggeringElement.classList.contains('r-btn-delete')) {
        hide(document.getElementById("selected-resource-display"));
        document.querySelector("#vfs-reload").dispatchEvent(new Event('click'));
      }
    } else {
      const feedback = document.querySelector('#vfs-feedback');
      feedback.classList.remove('hidden');
      feedback.textContent = event.detail.xhr.responseText.replace(/[{}]/g, '');
      feedback.classList.add('red');
      feedback.classList.remove('green');
      setTimeout(() => {
        feedback.classList.add('hidden');
        feedback.textContent = '';
      }, 8000);
    } 
  }
});

document.addEventListener('htmx:responseError', function(event) {
  if (event.detail.xhr.status === 401) { // token expired
     // Prevent HTMX from replacing content
     event.detail.shouldSwap = false;
     window.location.href = "/api/v1/login";
     event.preventDefault();
  }
});

document.addEventListener("htmx:configRequest", function(evt) {
  const triggerEl = evt.detail.elt;
  const triggeringElementId = triggerEl.id;

  // Only apply this to the upload form
  if (triggeringElementId === "upload-files-form-dash") {
    const spanVal = document.getElementById("selected-volume").textContent;
    evt.detail.headers['X-Volume-Target'] = spanVal;
  } else if (triggeringElementId === "job-create-form") {
    const loader = document.querySelector("#job-submit-loader");
    const form = document.querySelector("#job-create-form");
    loader.classList.remove("hidden");
    form.classList.add("disabled");
    setTimeout(() => {
      loader.classList.add("hidden");
      form.classList.remove("disabled");
    }, 2000);
  }

});

document.addEventListener("htmx:afterOnLoad", function(evt) {
  
});





// upload drop-zone status line (success / error), then back to the prompt
function kUploadState(cls, text, ms) {
  const box = document.querySelector("#file-boxes");
  const p = document.querySelector(".fupload-header > p");
  if (!box || !p) return;
  box.classList.remove("is-success", "is-error");
  box.classList.add(cls);
  p.textContent = text;
  clearTimeout(kUploadState.t);
  kUploadState.t = setTimeout(() => {
    box.classList.remove(cls);
    p.textContent = "Browse File to upload or drag & drop!";
  }, ms);
}
