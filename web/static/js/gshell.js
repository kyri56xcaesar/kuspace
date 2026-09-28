

// shell related
let shellCounter = 0;
// Create new DIV, assign unique ID, and append to spawner
function newTerminal() {
  if (shellCounter >= 5) {
    alert('Ok relax buddy, no more terms');
    return null;
  }
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
  terminalBody.scrollIntoView(false);
  // websocket: wss only admits the shared gshell room (jid 0, role "jack")
  // with a short-lived ticket from frontapp
  let socket = null;
  (async function connect() {
    let ticket;
    try {
      const r = await fetch("/api/v1/verified/ws-ticket?jid=0&role=jack", { credentials: "same-origin" });
      if (!r.ok) throw new Error(r.status);
      ticket = (await r.json()).ticket;
    } catch (e) {
      appendLine("Could not join gShell (no ticket): " + e.message);
      return;
    }
    const proto = location.protocol === "https:" ? "wss://" : "ws://";
    socket = new WebSocket(proto + WS_ADDRESS + "/get-session?role=jack&jid=0&ticket=" + encodeURIComponent(ticket));

    socket.onopen = function () {
      console.log("Connected to WebSocket server");
    };

    socket.onmessage = function (event) {
      appendLine(event.data);
      setTimeout(() => {
        terminalBody.scrollTop = terminalBody.scrollHeight;
      }, 100);
    };

    socket.onclose = function () {
      appendLine("Disconnected from gShell.");
    };
  })();




  // Enter runs a command (gshell-commands.js); "say ..." talks to the room.
  // Up/down walk the command history.
  const history = [];
  let hpos = 0;
  const scrollDown = () => setTimeout(() => { terminalBody.scrollTop = terminalBody.scrollHeight; }, 50);
  terminalInput.addEventListener('keydown', (event) => {
    if (event.key === 'ArrowUp' && hpos > 0) { terminalInput.value = history[--hpos]; event.preventDefault(); }
    if (event.key === 'ArrowDown') { hpos = Math.min(hpos + 1, history.length); terminalInput.value = history[hpos] || ""; event.preventDefault(); }
  });
  terminalInput.addEventListener('keypress', async (event) => {
    if (event.key === 'Enter') {
      const command = terminalInput.value;
      terminalInput.value = "";
      if (!command.trim()) return;
      history.push(command);
      hpos = history.length;
      appendLine(command, "k>");
      const verb = command.trim().split(/\s+/)[0];
      if (verb === "clear") {
        terminalBody.querySelectorAll(".line").forEach((l) => l.remove());
      } else if (verb === "say") {
        if (socket && socket.readyState === WebSocket.OPEN) socket.send(command.trim().slice(4));
        else appendLine("not connected to the room");
      } else if (typeof window.gshellRun === "function") {
        await window.gshellRun(command, (text) => appendLine(text));
      }
      scrollDown();
    }
  });

  // Lines are text, never HTML: the room relays other users' input, and
  // innerHTML let anyone run script in every connected user's page.
  function makeLine(text, prompt) {
    let line = document.createElement("div");
    line.classList.add("line");
    if (prompt) {
      let p = document.createElement("span");
      p.classList.add("gshell-prompt");
      p.textContent = prompt + " ";
      line.appendChild(p);
    }
    line.appendChild(document.createTextNode(text));
    return line;
  }

  function appendLine(text, prompt) {
    terminalBody.appendChild(makeLine(text, prompt));
  }

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
    terminalBody.style.width = (e.clientX - element.offsetLeft) + 'px';
    terminalBody.style.height = (e.clientY - element.offsetTop) + 'px';
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
    if (socket) socket.close();
    element.remove();
    shellCounter--;
  });

}
