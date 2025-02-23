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

function toggleDarkMode() {
  const elementsToToggle = document.querySelectorAll(".darkened");

  elementsToToggle.forEach((el) => {
    el.classList.toggle("dark-mode");
  });

  const darkMode = document.body.classList.contains("dark-mode");
  localStorage.setItem("darkMode", darkMode);
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

function showModal(modalId) {
  document.getElementById(modalId).classList.remove("hidden");
}
function closeModal(modalId) {
  document.getElementById(modalId).classList.add("hidden");
}

function copyToClipboard(selector) {
  const element = document.querySelector(selector);
  if (element) {
    const text = element.textContent.trim(); // Trim any extra spaces
    navigator.clipboard.writeText(text).then(() => {
      const copyBtn = document.getElementById("copy-btn");
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


// htmx events handling

//global variable of a list that will hold the users
let fetchedUsers = null;
let fetchedGroups = null;

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
  if (triggeringElementId === "gshell-spawner") {
    let newShell = newTerminal();
    event.detail.target = newShell;
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

  //console.log(triggeringElement.id);

  if (event.detail.xhr.status == 401 || event.detail.xhr.status == 303) {
    window.location.href = "/api/v1/login";
    location.reload(); 

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
  
  } else if (triggeringElement.id === 'load-users-to-cache') {
    if (event.detail.xhr.status >= 200 && event.detail.xhr.status < 300) {
      const rawResponse = event.detail.xhr.responseText;
      try {
        fetchedUsers = JSON.parse(rawResponse); 
        console.log("Fetched users:", fetchedUsers);
      } catch (error) {
        console.error("Could not parse JSON:", error, rawResponse);
      }
    }
  
  } else if (triggeringElement.id === 'load-groups-to-cache') {
    if (event.detail.xhr.status >= 200 && event.detail.xhr.status < 300) {
      const rawResponse = event.detail.xhr.responseText;
      try {
        fetchedGroups = JSON.parse(rawResponse); 
        console.log("Fetched groups:", fetchedUsers);
      } catch (error) {
        console.error("Could not parse JSON:", error, rawResponse);
      }
    }
  
  } else if (triggeringElement.id === 'reload-btn') {
  
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
      window.location.href="/api/v1/login";
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
        hideProgressBar(document.getElementById('progress-container'))
      }, 2000);
      document.getElementById('upload-files-form').reset();
      resetFileBoxDisplay();

      if (event.detail.xhr.status >= 200 && event.detail.xhr.status < 300) {
        while(filesList.length > 0) {
          filesList.pop();
        }
        const feedback = document.querySelector(".fupload-header > svg");
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
          hideProgressBar(document.querySelector(".r-loader"));
        }, 4000);
      } else {
        const feedback = document.querySelector(".feedback");
        const msg = document.createElement("p");
        msg.textContent = "Failure";
        msg.style.color = "red";
        feedback.appendChild(msg);

        setTimeout(() => {
          msg.remove();
          hideProgressBar(document.querySelector(".r-loader"));
        }, 4000);
      }
  } else if (triggeringElement.id === 'root-dashboard-loader') {
      if (event.detail.xhr.status >= 200 && event.detail.xhr.status < 300) {
        const profmenu = document.querySelector(".profile-menu");
        profmenu.remove();
        const toggleButton = document.querySelectorAll("#root-dashboard-loader .toggle-button-collapse");
        console.log(toggleButton);
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



// shell related
let shellCounter = 0;

// Create new DIV, assign unique ID, and append to spawner
function newTerminal() {
  shellCounter++;
  let uniqueId = 'gshell-container-' + shellCounter;

  let newShell = document.createElement('div');
  newShell.classList.add('gshell-container');
  newShell.setAttribute('id', uniqueId);


  const spawner = document.getElementById('gshell-spawner');
  spawner.appendChild(newShell);

  return newShell;
}


function giveFunctionality(element) {
  if (!element) {
    return;
  }
  const terminalBody = element.querySelector('#terminal-body');
  const terminalInput = element.querySelector('#terminal-input');

  // Listen for the Enter key to process commands
  terminalInput.addEventListener('keypress', (event) => {
    if (event.key === 'Enter') {
      const inputValue = terminalInput.value.trim();
      // 1. Create a new line to display the entered command
      const newLine = document.createElement('div');
      newLine.classList.add('line');
      newLine.textContent = '$ ' + inputValue;
      // Insert above the current input line
      terminalBody.insertBefore(newLine, terminalInput.parentElement.nextSibling);
      // 2. Clear the input
      terminalInput.value = '';
      // 3. You can replace this with actual command logic
      // For now, just echo back what was typed
      if (inputValue) {
        const responseLine = document.createElement('div');
        responseLine.classList.add('line');
        responseLine.textContent = 'You typed: ' + inputValue;
        terminalBody.insertBefore(responseLine, terminalInput.parentElement.nextSibling);
      }
    }
  });

  // ===== DRAGGING =====
  const terminal = element.querySelector('.terminal');
  const terminalHeader = element.querySelector('.terminal-header > .draggable-bar');

  let offsetX = 0;
  let offsetY = 0;
  let isDragging = false;
  terminalHeader.addEventListener('mousedown', (e) => {
    // Calculate the distance between the mouse pointer and the container's top-left corner
    offsetX = e.clientX - element.offsetLeft;
    offsetY = e.clientY - element.offsetTop;
    isDragging = true;
 
    // Add global listeners so dragging works even if the mouse leaves the header
    document.addEventListener('mousemove', onMouseMove);
    document.addEventListener('mouseup', onMouseUp);
  });
 
  function onMouseMove(e) {
    if (!isDragging) return;
    e.preventDefault();
    // Move the container so it follows the mouse pointer
    element.style.left = (e.clientX - offsetX) + 'px';
    element.style.top  = (e.clientY - offsetY) + 'px';
  }
 
  function onMouseUp(e) {
    isDragging = false;
    document.removeEventListener('mousemove', onMouseMove);
    document.removeEventListener('mouseup', onMouseUp);
  }
 
  // ===== RESIZING =====
  const resizer = element.querySelector("#resizer");
  let isResizing = false;
 
  resizer.addEventListener('mousedown', (e) => {
    e.preventDefault(); // Prevent text selection
    isResizing = true;
    document.addEventListener('mousemove', onResize);
    document.addEventListener('mouseup', stopResize);
  });
 
  function onResize(e) {
    if (!isResizing) return;
    e.preventDefault();
    // Adjust width/height based on mouse position
    terminal.style.width  = (e.clientX - element.offsetLeft) + 'px';
    terminal.style.height = (e.clientY - element.offsetTop)  + 'px';
  }
 
  function stopResize(e) {
    isResizing = false;
    document.removeEventListener('mousemove', onResize);
    document.removeEventListener('mouseup', stopResize);
  }

  element.querySelector(".minimize").addEventListener('click', ()=> {
    console.log('minimizing');
  });

  element.querySelector(".pin").addEventListener('click', ()=> {
    console.log('pinning');
  });

  element.querySelector(".close").addEventListener('click', ()=> {
    element.remove();
  });

}
