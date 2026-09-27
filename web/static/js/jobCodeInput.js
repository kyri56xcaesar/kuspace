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
    pypandas: "python",
    octave: "text",
    caengine: "text",
    bash: "text/x-sh",
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
  pypandas: `# example\n
df.columns = df.columns.str.strip().str.lower().str.replace(" ", "_")\n
df = df.rename(columns={"col1": "col1_renamed"})
  `,
  octave: `# example\n
input(:,2) += 5;\n
output = input;
  `,
  ffmpeg: `# example' extract audio then recombine with different bitrate\n
ffmpeg -i {input} -vn -acodec copy audio.aac && ffmpeg -i {input} -i audio.aac -c:v copy -c:a aac -b:a 128k {output}
  `,
  caengine: `states: 2\ngenerations: 20\nneighborhood: [[1,1,1],[1,0,1],[1,1,1]]\n`,
  bash: `# example, sort by the second column\ntail -n +2 {input} | sort -t, -k2 -nr > {output}`,
  go:"func run(data string) string {return data}\n",
  c:"void run(char *buffer) {}\n",
  java:"public static String run(String data) {return data;}\n",
  duckdb:"--example\nCREATE TABLE test_data AS SELECT * FROM {input};\nSELECT * FROM test_data LIMIT 5;\n",
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
 
 
function appendJobLine(div, jid, text) {
  const message = document.createElement("p");
  const prefixSpan = document.createElement("span");
  prefixSpan.textContent = "job-" + jid + ":\t";
  prefixSpan.classList.add("blue");
  const messageSpan = document.createElement("span");
  messageSpan.textContent = text;
  message.appendChild(prefixSpan);
  message.appendChild(messageSpan);
  div.appendChild(message);
  div.scrollTop = div.scrollHeight;
}

// showSavedJobLog prints a job's saved output (for jobs whose live stream is
// over or unreachable).
async function showSavedJobLog(jid, div) {
  try {
    const r = await fetch("/api/v1/verified/job-log?jid=" + encodeURIComponent(jid), { credentials: "same-origin" });
    if (!r.ok) {
      return false;
    }
    const text = await r.text();
    if (!text) {
      return false;
    }
    text.split("\n").filter(Boolean).forEach((line) => appendJobLine(div, jid, line));
    return true;
  } catch (e) {
    return false;
  }
}

// createFeedbackPanel streams a job's live output. wss only accepts
// connections carrying a short-lived ticket, which frontapp issues after
// checking the job is ours.
async function createFeedbackPanel(jid, div) {
  if (!div) {
    return;
  }
  let ticket;
  try {
    const r = await fetch("/api/v1/verified/ws-ticket?jid=" + encodeURIComponent(jid), { credentials: "same-origin" });
    if (!r.ok) {
      throw new Error("ticket refused: " + r.status);
    }
    ticket = (await r.json()).ticket;
  } catch (e) {
    console.error("cannot watch job", jid, e);
    if (!(await showSavedJobLog(jid, div))) {
      appendJobLine(div, jid, "live output is not available for this job");
    }
    return;
  }

  const scheme = location.protocol === "https:" ? "wss://" : "ws://";
  const socket = new WebSocket(scheme + WS_ADDRESS + "/get-session?jid=" + encodeURIComponent(jid) +
    "&role=consumer&ticket=" + encodeURIComponent(ticket));
  let received = false;
  socket.onmessage = (event) => {
    received = true;
    appendJobLine(div, jid, event.data);
  };
  socket.onopen = () => console.log("Connected to Jobs Websocket server");
  socket.onclose = async () => {
    console.log("Disconnected from Jobs Websocket server");
    // nothing came through live (e.g. the job had already finished): show
    // what was saved instead
    if (!received) {
      await showSavedJobLog(jid, div);
    }
  };
}

function normalizeIndentation(code) {
    return code
        .split("\n")
        .map(line => line.replace(/^\t+/, match => ' '.repeat(match.length * 4))) // convert tabs to spaces
        .join("\n");
}
 
 
