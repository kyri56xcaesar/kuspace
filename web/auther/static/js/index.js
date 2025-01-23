let domReady = (cb) => {
  document.readyState === 'interactive' || document.readyState === 'complete'
    ? cb()
    : document.addEventListener('DOMContentLoaded', cb)
};

domReady(() => {
// Display body when DOM is loaded 
  document.body.style.visibility = 'visible';

  // attach the closing of infos/tips/warnings to the buttons 
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
});

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

document.addEventListener('htmx:afterSwap', function (event) {
  const verifyResultElement = document.getElementById('verify-result');
  if (verifyResultElement) {
    const result = verifyResultElement.textContent.trim();
    if (result.toLowerCase() === 'true') {
      verifyResultElement.className = 'true';
    } else if (result.toLowerCase() === 'false') {
      verifyResultElement.className = 'false';
    }
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

  if (event.detail.xhr.status == 401) {
    window.location.href = "/api/v1/login";
    return;
  }
  // Handle different cases
  // all users fetch
  if (triggeringElement.id === 'fetch-users-results') { 
    if (event.detail.xhr.status >= 300 && event.detail.xhr.status < 400) {
      const redirectLocation = event.detail.xhr.getResponseHeader("Location");
      if (redirectLocation) {
        window.location.href = redirectLocation;
      } else {
        console.error("Redirect location not found in the response."); 
      }    
    }
    // reload users fetch
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
    }x
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
    } else if (event.detail.xhr.status >= 400 && event.detail.xhr.status < 500) {
    } else {
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
