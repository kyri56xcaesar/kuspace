 // what we need to prepare jobs
const modeMap = {
    // "js": "javascript",
    // "go": "go",
    // "py": "python",
    // "java": "java",
    // "c": "gcc",
    // "javascript":"node",

    duckdb: "text/x-sql",   // duckdb is SQL-based
    sql: "text/x-sql",
    python: "python",
    custom: "python"
  };
  
const extMap = {
  "js":"javascript",
  "py":"python",
  "go":"go",
  "c":"c",
  "java":"openjdk",
  "sql":"sql",
};

const defaultMap = {
  sql:"-- SELECT * FROM #% WHERE ;\n",
  javascript:"function run(data) {return data}\n",
  python:`
def run(data):\n\treturn data
`,
  go:"func run(data string) string {return data}\n",
  c:"void run(char *buffer) {}\n",
  java:"public static String run(String data) {return data;}\n",
  duckdb:"--example\nCREATE TABLE test_data AS SELECT * FROM #%;\nSELECT * FROM test_data LIMIT 5;\n",
  sql:"SELECT * FROM table_name;\n",
}

const MAX_SIZE_MB = 1;
const MAX_SIZE_BYTES = MAX_SIZE_MB * 1024 * 1024;

function setupJobSubmitter(element) {
    // "JOB" preperation setup
    const editor = CodeMirror.fromTextArea(element.querySelector(".code-editor"), {
      lineNumbers: true,
      theme: "monokai",  
      matchBrackets: true,
      autoCloseBrackets: true,
      smartIndent: false,       // disables smart indent on newlines
      mode: "python",

    });

    setTimeout(() => {
      editor.setValue(defaultMap["duckdb"]);

    }, 1000);


    const codeSnipperUpload = element.querySelector("#code-file-upload");
    const appSelect = element.querySelector("#language-selector");

    appSelect.addEventListener("change", function() {
      const selectedValue = this.value.toLowerCase();
      const mode = modeMap[selectedValue];
      console.log('mode: ' + mode);
      console.log('selectedValue: ' + selectedValue);
      editor.setOption("mode", mode);
      editor.setValue(defaultMap[selectedValue]);
      editor.refresh(); 
    });

    codeSnipperUpload.addEventListener("change", function() {
      const file = event.target.files[0];
      if (!file) return;

      if (file.size > MAX_SIZE_BYTES) {
        alert(`File too large. Max allowed size is ${MAX_SIZE_MB}MB.`);
        return;
      }

      const ext = file.name.split('.').pop() || ""; 
      const mode = modeMap[ext] || "python"; 
      editor.setOption("mode", mode);    
      // console.log('mode: ' + mode);
      appSelect.value = extMap[ext];

      const reader = new FileReader();
      reader.readAsText(file);
      reader.onload = function(event) {
        editor.setValue(event.target.result);
      };
      reader.onerror = function(event) {
        console.error("Error reading file:", event.target.error);
      };
      editor.refresh(); 
    });

    setTimeout(() => {
      editor.setValue(defaultMap[""] || "");
      editor.refresh(); 
    }, 100);



   

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
 
 
