 // what we need to prepare jobs
const modeMap = {
    "js": "javascript",
    "go": "go",
    "py": "python",
    "java": "java",
    "c": "gcc",
    "javascript":"node",
    "python":"python",
  
  };
  
const extMap = {
  "js":"javascript",
  "py":"python",
  "go":"go",
  "c":"c",
  "java":"openjdk",
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
def run(data):\n\treturn data






`,
  "go":
`
func run(data string) string {
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
function setupJobSubmitter(element) {
    // "JOB" preperation setup
    element.querySelector("#language-selector").value = "python"; 
    // reset text area 
    element.querySelector("#j-description").value = "";
    // html/css/js mini "code editor"
    // Load CodeMirror
    // console.log(element.querySelector("#code-editor"));
    const editor = CodeMirror.fromTextArea(element.querySelector(".code-editor"), {
      mode: "python", // Default mode
      lineNumbers: true,
      theme: "monokai",  // Choose a theme
      matchBrackets: true,
      autoCloseBrackets: true,
      // indentUnit: 0,            // size of a single indent
      smartIndent: false,       // disables smart indent on newlines
      // indentWithTabs: false,    // do not use tabs for indentation
      // extraKeys: {
      //   Tab: false              // disable tab behavior
      // }

    });


    setTimeout(() => {
      editor.setValue(defaultMap["python"] || "");
      editor.refresh(); // <-- this line saves lives
      element.querySelector("#submit-job-button").checked = true;
    }, 100);
    // console.log(editor);

    // Language selection logic
    element.querySelector("#language-selector").addEventListener("change", function() {
      const mode = modeMap[this.value];
      editor.setOption("mode", mode);
      editor.setValue(defaultMap[this.value]);
    });

    // Code file upload logic
    element.querySelector("#code-file-upload").addEventListener("change", function(event) {
      const file = event.target.files[0];
      if (!file) return;

      const ext = file.name.split('.').pop(); 
      const mode = modeMap[ext] || "python"; 
      // console.log('mode: ' + mode);
      element.querySelector("#language-selector").value = extMap[ext];

      const reader = new FileReader();
      editor.setOption("mode", mode);    

      reader.onload = function(e) {
        editor.setValue(e.target.result);
      };
      reader.readAsText(file);
    });

    // specify output functionality
    element.querySelector("#select-output-destination").addEventListener("input", function(event) {
      const inputValue = event.target.value;
      const spanElement = event.target.closest('div').parentElement.nextElementSibling.children[4];
      spanElement.textContent = inputValue;
    });

    // select input "resources" for the job display handler
    element.querySelector("#select-j-input-button").addEventListener("click", function(event) {
      // resource selection modal
      const existingModal = element.querySelector("#resource-selection-modal");
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
      document.querySelector("#submit-resource-selection").addEventListener("click", function () {
        const selectedResources = [];
        document.querySelectorAll("#resource-selection-table input[type='checkbox']:checked").forEach((checkbox) => {
            selectedResources.push({
                id: checkbox.getAttribute("data-resource-id"),
                name: checkbox.getAttribute("data-resource-name"),
            });
        });

        // You can send selectedResources to another function or API
        //alert(`Selected ${selectedResources.length} resources!`);
        element.querySelector(".input-box").textContent = selectedResources.map(resource => `${resource.name}`).join('\n');

        // Close the modal
        modalOverlay.remove();
      });

      // Handle cancel
      document.querySelector("#cancel-resource-selection").addEventListener("click", function () {
          modalOverlay.remove();
      });



    });

    setTimeout(() => {
        editor.setValue(defaultMap["python"] || "");  // Prevent undefined values
        element.querySelector("#submit-job-button").checked = true;
    }, 100);

    // Job submission, lets do it as a promise, more flexible for this rather than htmx
    const submitJobBtn = element.querySelector("#submit-job-button");

    submitJobBtn.addEventListener("change", function(event) {
        if (submitJobBtn.checked) {// cancel job case {}
        //confirm?
            if (!confirm("Are you sure you want to cancel the job execution?")) {
                submitJobBtn.checked = false;
                return;
            }

            // start an indicator spinner
            const jloader = element.querySelector('.j-loader');
            jloader.classList.remove("hidden");
            jloader.style.animation="reverseSpin var(--speed) infinite linear";
            // send the request and await response (maybe trigger a ws to get realtime data about the job)
            // handle response, display, spinner, output
 
        } else { // send job
            // start an indicator spinner
            const jloader = element.querySelector('.j-loader');
            jloader.style.animation="spin var(--speed) infinite linear";
            jloader.classList.remove("hidden");
    

            // send job request
            // job data 
            const input = element.querySelector(".input-box").textContent.split('\n').map((file) => file.replace(/^\/+/, ''));
            const output = element.querySelector(".output-box").textContent;
            const code = normalizeIndentation(editor.getValue());
            const logic = editor.getOption("mode");
            const description = element.querySelector("#j-description").value;

            // verify logic integrity
            // gather data
            let job = {
              "uid":0,
              "input":input,
              "output":output,
              "logic":logic,
              "logic_body":code,
              "description":description,
            }
        
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
              // console.log(resp);
              // we get the job id here, and we can open a feedback panel for this job here..
              // open feedback panel
              if (resp.status == "error") {
                return;
              } else {
                const jobFeedbackPanel = element.querySelector('#job-feedback');
                jobFeedbackPanel.classList.remove('hidden');
                createFeedbackPanel(resp.jid);
            
              }
          
          
          
            }).catch(error => {
              console.error('Error fetching resources:', error);
            });
        
            setTimeout(() => {
                jloader.classList.add('hidden');
                submitJobBtn.checked = true;
            }, 2000);

        }
    });

    // job i/o bar minimizer 
    element.querySelector("#job-io-minimizer").addEventListener("click", (event) => {
      const ioSetupDiv = document.getElementById("job-io-setup");
      // ioSetupDiv.classList.toggle("minimized");

      ioSetupDiv.querySelectorAll(".minimizable").forEach((div) => {
        div.classList.toggle("hidden");
        div.classList.toggle("minimized");
      })
    });

    // job feedback bar minimizer 
    element.querySelector("#job-feedback-minimizer").addEventListener("click", (event) => {
      const feedbackDiv = document.getElementById("job-feedback");
      feedbackDiv.classList.toggle("minimized");

      feedbackDiv.querySelectorAll(".minimizable").forEach((div) => {
        div.classList.toggle("hidden");
        div.classList.toggle("minimized");
      })
    });

    return editor;

 }
 
 
function createFeedbackPanel(jid) {
    // create the display element, what should it be?
    const feedbackMessagesDiv = document.getElementById("feedback-messages");
    feedbackMessagesDiv.classList.add("minimizable");

    const socket = new WebSocket('ws://'+IP+':8082/job-stream?jid='+jid+'&role=Consumer');

    const prefix = "job-"+jid+":\t";
    socket.onmessage = (event) => {
        const message = document.createElement("p");
        const prefixSpan = document.createElement("span");
        prefixSpan.textContent = prefix;
        prefixSpan.classList.add("blue");
 
        const messageSpan = document.createElement("span");
        messageSpan.textContent = event.data;
 
        message.appendChild(prefixSpan);
        message.appendChild(messageSpan);
        feedbackMessagesDiv.appendChild(message);
        feedbackMessagesDiv.scrollTop = feedbackMessagesDiv.scrollHeight;
    };
    socket.onopen = function () {
        console.log("Connected to Jobs Websocket server");
    };
    socket.onclose = (event) => {
        console.log("Disconnected from Jobs Websocket server");
    }
 }
  
function normalizeIndentation(code) {
    return code
        .split("\n")
        .map(line => line.replace(/^\t+/, match => ' '.repeat(match.length * 4))) // convert tabs to spaces
        .join("\n");
}
 
 
