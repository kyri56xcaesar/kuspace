// unused currently
cachedUsers = [];
cachedGroups = [];
cachedResources = [];
const IP = "localhost";
const PORT ="8080"


let domReady = (cb) => {
  document.readyState === 'interactive' || document.readyState === 'complete'
    ? cb()
    : document.addEventListener('DOMContentLoaded', cb)
};
domReady(() => {
  document.body.style.visibility = 'visible';

  // TIPS/INFO BUTTONS
  const toggleButton = document.querySelectorAll(".toggle-button-collapse");
  toggleButton.forEach(toggleButton => {
    toggleButton.addEventListener("click", () => {
      toggleButton.classList.toggle("collapsed");
      // get the closest h1 or p or span...
      target = toggleButton.closest(".info").querySelector(".target");
      target.classList.toggle("collapsed");  
      if (toggleButton.classList.contains("collapsed")) {
        toggleButton.style.transform = `translateX(-${target.offsetWidth}px)`;
      } else {
        toggleButton.style.transform = `translateX(0)`;
      }
    });
  });

  const logout_btn = document.getElementById("logout-a");
  if (logout_btn) {
    logout_btn.addEventListener("click", () => {
      console.log("logging out");
      // window.location.href = "http://"+IP+":"+PORT+"/api/v1/login";
    });
  }

  // dark mode memory
  const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
  sleep(1000).then(() => {
    const tglBtn = document.getElementById("dark-mode-toggle");
    if (tglBtn) {
      const elementsToToggle = document.querySelectorAll(".darkened");

      if (localStorage.getItem("darkMode") === "true") {
        elementsToToggle.forEach((el) => {
          el.classList.add("dark-mode");
        });
        tglBtn.checked = true;
      } else {
        elementsToToggle.forEach((el) => {
          el.classList.remove("dark-mode");
        });
        tglBtn.checked = false;
      }
    }
  });
});

function toggleDarkMode() {
  const elementsToToggle = document.querySelectorAll(".darkened");

  elementsToToggle.forEach((el) => {
    el.classList.toggle("dark-mode");
  });

  const darkMode = document.body.classList.contains("dark-mode");
  localStorage.setItem("darkMode", darkMode);
}

function toggleCollapses() {
  const collapsibleElmnts = document.querySelectorAll(".collapsible");
  collapsibleElmnts.forEach((el) => {
    if (el.tagName === "BUTTON") {
      el.click();
    } else {
      el.classList.toggle("collapsed");
    }
  });
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

function showSubSection(sectionId) {
  const subsections = document.querySelectorAll('.subsection');
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


function copyToClipboard(selector, copyBtnId) {
  const element = document.querySelector(selector);
  if (element) {
    const text = element.textContent.trim(); // Trim any extra spaces
    navigator.clipboard.writeText(text).then(() => {
      const copyBtn = document.getElementById(copyBtnId);
      if (copyBtn) {
        copyBtn.textContent = "✔️"; // Show a checkmark temporarily
        setTimeout(() => {
          copyBtn.textContent = "📋"; // Revert back to clipboard icon
        }, 2000); // Reset after 2 seconds
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

function downloadResource(filename) {
  const link = document.createElement("a");
  link.href = `/api/v1/verified/download?target=${(filename)}`;
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



// htmx events handling
document.addEventListener('htmx:afterSettle', function(event) {
  const triggeringElement = event.detail.elt;
  const triggeringElementId = triggeringElement.id;
  if (triggeringElementId === 'fetch-users-results')  {
    // the dark mode part for all reload/partial html fethc
    if (localStorage.getItem("darkMode") === "true") {
      const table = event.detail.target.querySelector('#all-users-table');
      if (table) {
        table.classList.add('dark-mode');
      }
    }
  } else if (triggeringElementId === 'fetch-groups-results') {
    // the dark mode part for all reload/partial html fethc
    if (localStorage.getItem("darkMode") === "true") {
      const table = event.detail.target.querySelector('#all-groups-table');
      if (table) {
        table.classList.add('dark-mode');
      }
    }
  }
})

document.addEventListener('htmx:beforeSwap', function(event) {
  const triggeringElement = event.detail.elt;
  const triggeringElementId = triggeringElement.id;

  if (event.detail.xhr.status === 401) {
    // Prevent HTMX from replacing content
    event.detail.shouldSwap = false;
    
    // alert('Your session has expired. Redirecting to login...');
    window.location.href = "http://"+IP+":"+PORT+"/api/v1/login";
    event.preventDefault();
  }
  
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
  } else if (triggeringElementId === "fetch-jobs-div" || triggeringElementId === "job-search-by-select") {
    cacheJobResultsLi = document.getElementById("fetch-jobs-div").querySelectorAll("li");
    // console.log(cacheJobResultsLi);
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

  if (event.detail.xhr.status == 401 || event.detail.xhr.status == 303) {
    event.preventDefault();
    
    window.location.href = "http://"+IP+":"+PORT+"/api/v1/login";

    return;
  }
  // Handle different cases
  // all users fetch
  if (triggeringElement.id === 'fetch-users-results') { 
    if (event.detail.xhr.status >= 300 && event.detail.xhr.status < 400) {
      const redirectLocation = event.detail.xhr.getResponseHeader("Location");
      if (redirectLocation) {
        window.location.href = redirectLocation;
      } else if (event.detail.xhr.status >= 200 && event.detail.xhr.status < 300) {

      } else {
        console.error("Redirect location not found in the response."); 
      }  
    }
    // reload users fetch
  
  } else if (triggeringElement.id === 'fetch-groups-results') {
    if (event.detail.xhr.status >= 200 && event.detail.xhr.status < 300) {

    }
  
  }  else if (triggeringElement.id === 'reload-btn') {
  
  } else if (triggeringElement.id === 'add-user-form') {
    // new user creation (from admin)
    if (event.detail.xhr.status >= 200 && event.detail.xhr.status < 300) {
      document.getElementById('reload-btn').dispatchEvent(new Event('click'));
      triggeringElement.reset();
    } else if (event.detail.xhr.status < 400) {
    } else if (event.detail.xhr.status < 500) {
      const form = triggeringElement.closest('form');
      form.classList.add('error-highlight');
      const feedback = document.getElementById('useradd-error-feedback');
      feedback.textContent = event.detail.xhr.responseText.replace(/[{}]/g, '');
      setTimeout(()=>{
        form.classList.remove('error-highlight');
        feedback.textContent = '';
      }, 40000);
    }
  // deleting users by admin 
  
  
  } else if (triggeringElement.id.startsWith('delete-btn-')) {
    if (event.detail.xhr.status >= 200 && event.detail.xhr.status < 300) {
      // Successfully deleted
      const rowId = triggeringElement.closest('tr').id; // Get the table row ID
      document.getElementById(rowId).remove(); // Remove the table row
      document.getElementById('reload-btn').dispatchEvent(new Event('click')); 
    } else {
      // Failed delete, apply red border
      const rowId = triggeringElement.closest('tr').id;
      document.getElementById(rowId).style.border = '2px solid red';
    }
  
  
  } else if (triggeringElement.id.startsWith('delete-grp-btn')) {
    if (event.detail.xhr.status >= 200 && event.detail.xhr.status < 300) {
      // Successfully deleted
      const rowId = triggeringElement.closest('tr').id; // Get the table row ID
      document.getElementById(rowId).remove(); // Remove the table row
      document.getElementById('reload-groups-btn').dispatchEvent(new Event('click')); 
    } else {
      // Failed delete, apply red border
      const rowId = triggeringElement.closest('tr').id;
      document.getElementById(rowId).style.border = '2px solid red';
    }
  // logging out (generic)
 
  
  } else if (triggeringElement.id.startsWith("logout")) {
    if (event.detail.xhr.status >= 200 && event.detail.xhr.status < 400) {
      window.location.href = "http://"+IP+":"+PORT+"/api/v1/login";
    }
  // hasher related
 

  } else if (triggeringElement.id.startsWith("inp-text")) {
    if (event.detail.xhr.status >= 400) {
      document.getElementById("generated-hash").textContent = "";
    }  


  // submit user patch by admin
  } else if (triggeringElement.id.startsWith("submit-btn-")) {
    if (event.detail.xhr.status >= 200 && event.detail.xhr.status < 300) {
      const row = triggeringElement.closest('tr');
      if (row) {
        row.classList.add('check-highlight');
        setTimeout(()=>{
          row.classList.remove('check-highlight')
        }, 2000);
      }
      document.getElementById('reload-btn').dispatchEvent(new Event('click'));
    }else if (event.detail.xhr.status >= 500 || event.detail.xhr.status == 400){
      const row = triggeringElement.closest('tr');
      if (row) {
        row.classList.add('error-highlight');
        setTimeout(()=>{
          row.classList.remove('error-highlight');
        }, 2000);
      }
    } else if (event.detail.xhr.status == 404) {
        const row = triggeringElement.closest('tr');
        if (row) {
          row.classList.add('warning-highlight');
          setTimeout(()=>{
            row.classList.remove('warning-highlight')
          }, 2000);
        }
    }
 

  } else if (triggeringElement.id === 'add-group-form') {
    if (event.detail.xhr.status >= 200 && event.detail.xhr.status <300) {
      document.getElementById('reload-groups-btn').dispatchEvent(new Event('click'));
      triggeringElement.reset();
    } 
    

  
  } else if (triggeringElement.id === 'upload-files-form') {
      setTimeout(() => {
        hide(document.getElementById('progress-container'))
      }, 2000);


      if (event.detail.xhr.status >= 200 && event.detail.xhr.status < 300 && fileUploadModule) {
        fileUploadModule.reset();

        const feedback = document.querySelector("#file-boxes");
        feedback.style.opacity = "1";
        feedback.style.color = "green";
        const p = document.querySelector(".fupload-header > p");
        p.textContent = "File(s) uploaded";
        p.style.opacity = "1";
        setTimeout(() => {
          feedback.opacity = "0.4";
          p.style.opacity = "0.4";
          feedback.style.color = "black";
          p.textContent = "Browse File to upload or drag & drop!";
        }, 10000);  
        // reload resources
        document.querySelector("#fetch-resources-form").requestSubmit();
        document.getElementById("fetch-resources-form").scrollTo({ top: 0, behavior: "smooth"});

      } else if (event.detail.xhr.status >= 300) {
        const feedback = document.querySelector(".fupload-header > svg");
        feedback.style.opacity = "1";
        feedback.style.color = "red";
        const p = document.querySelector(".fupload-header > p");
        p.textContent = "Failed to upload.";
        p.style.opacity = "1";
        setTimeout(() => {
          feedback.opacity = "0.4";
          feedback.style.color = "black";
          p.textContent = "Browse File to upload or drag & drop!";
        }, 2000)
      }
  } else if (triggeringElement.id === 'upload-files-form-dash') {
      setTimeout(() => {
        hide(document.getElementById('progress-container'))
      }, 2000);


      if (event.detail.xhr.status >= 200 && event.detail.xhr.status < 300 && fileUploadModule) {
        fileUploadModule.reset();

        const feedback = document.querySelector("#file-boxes");
        feedback.style.opacity = "1";
        feedback.style.color = "green";
        const p = document.querySelector(".fupload-header > p");
        p.textContent = "File(s) uploaded";
        p.style.opacity = "1";
        setTimeout(() => {
          feedback.opacity = "0.4";
          p.style.opacity = "0.4";
          feedback.style.color = "black";
          p.textContent = "Browse File to upload or drag & drop!";
        }, 10000);  
        // reload resources
        // document.querySelector("#fetch-resources-form").requestSubmit();
        document.getElementById("fetch-resources-form").scrollTo({ top: 0, behavior: "smooth"});

      } else if (event.detail.xhr.status >= 300) {
        const feedback = document.querySelector(".fupload-header > svg");
        feedback.style.opacity = "1";
        feedback.style.color = "red";
        const p = document.querySelector(".fupload-header > p");
        p.textContent = "Failed to upload.";
        p.style.opacity = "1";
        setTimeout(() => {
          feedback.opacity = "0.4";
          feedback.style.color = "black";
          p.textContent = "Browse File to upload or drag & drop!";
        }, 2000)
      }
  } else if (triggeringElement.className === "r-btn-delete") {
      if (event.detail.xhr.status >= 200 && event.detail.xhr.status < 300) {
        document.querySelector("#fetch-resources-form").requestSubmit();
        document.getElementById("fetch-resources-form").scrollTo({ top: 0, behavior: "smooth"});

        const feedback = document.querySelector(".feedback");
        const msg = document.createElement("p");
        msg.textContent = "Success";
        msg.style.color = "green";
        feedback.appendChild(msg);

        // remove the selected 
        tableRows = document.querySelectorAll("#resource-list-table tbody tr");
        resourceDetails = document.getElementById("resource-details");

        tableRows.forEach((row) => {
            // Remove 'selected' class from all rows
            tableRows.forEach((r) => r.classList.remove("selected"));
        });
        resourceDetails.innerHTML ="";

        setTimeout(() => {
          msg.remove();
          hide(document.querySelector(".r-loader"));
        }, 4000);
      } else {
        const feedback = document.querySelector(".feedback");
        const msg = document.createElement("p");
        msg.textContent = "Failure";
        msg.style.color = "red";
        feedback.appendChild(msg);

        setTimeout(() => {
          msg.remove();
          hide(document.querySelector(".r-loader"));
        }, 4000);
      }
  } else if (triggeringElement.id === 'root-dashboard-loader') {
      if (event.detail.xhr.status >= 200 && event.detail.xhr.status < 300) {
        const profileMenu = document.querySelector(".profile-menu");
        const profileButton = document.querySelector(".profile-button");
        profileButton.addEventListener("click", () => {
            console.log("test");
            profileMenu.classList.toggle("open");
        });
        document.addEventListener("click", (event) => {
          if (!profileMenu.contains(event.target)) {
            profileMenu.classList.remove("open");
          }
        });
      }
  } else if (triggeringElement.id === 'permissionsInput' || triggeringElement.id === 'resource-path-select' || triggeringElement.id === 'resource-owner-select' || triggeringElement.id === 'resource-group-select') {
      if (event.detail.xhr.status >= 200 && event.detail.xhr.status < 300) {
        
      } else {

      }
  } else if (triggeringElement.id === 'preview-resource-btn') {
    if (event.detail.xhr.status >= 200 && event.detail.xhr.status < 300) {
      document.querySelector(".resource-preview-main").classList.remove("blurred");
      document.querySelector("#preview-resource-btn").remove();
    }
  } else if (triggeringElement.id === 'register-form') {
    if (event.detail.xhr.status < 300) {
      
    }
  }  else if (triggeringElement.id === 'load-users-to-cache') {
    if (event.detail.xhr.status >= 200 && event.detail.xhr.status < 300) {
      const rawResponse = event.detail.xhr.responseText;
      try {
        cachedUsers = JSON.parse(rawResponse); 
        console.log("Fetched users:", cachedUsers);
      } catch (error) {
        console.error("Could not parse JSON:", error, rawResponse);
      }
    }
  
  } else if (triggeringElement.id === 'load-groups-to-cache') {
    if (event.detail.xhr.status >= 200 && event.detail.xhr.status < 300) {
      const rawResponse = event.detail.xhr.responseText;
      try {
        cachedGroups = JSON.parse(rawResponse); 
        console.log("Fetched groups:", cachedGroups);
      } catch (error) {
        console.error("Could not parse JSON:", error, rawResponse);
      }
    }
  } else if (triggeringElement.id === 'load-resources-to-cache') {
    if (event.detail.xhr.status >= 200 && event.detail.xhr.status < 300) {
      const rawResponse = event.detail.xhr.responseText;
      try {
        cachedResources = JSON.parse(rawResponse); 
        console.log("Fetched resources:", cachedResources);
      } catch (error) {
        console.error("Could not parse JSON:", error, rawResponse);
      }
    }
  }
});

document.addEventListener('htmx:confirm', function(evt) {
  if (evt.target.matches("[confirm-with-sweet-alert='true']")) {
    evt.preventDefault();
    swal({
      title: "Are you sure?",
      text: "Are you sure you are sure?",
      icon: "warning",
      buttons: true,
      dangerMode: true,
    }).then((confirmed) => {
      if (confirmed) {
        evt.detail.issueRequest();
      }
    });
  }
});

document.addEventListener('htmx:responseError', function(event) {
  if (event.detail.xhr.status === 401) {
      // alert('Your session has expired. Redirecting to login...');
      window.location.href = "http://"+IP+":"+PORT+"/api/v1/login";
    }
});




