

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
  // websocket 
  const socket = new WebSocket("ws://"+WS_ADDRESS+"/get-session?role=jack&jid=0");

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
  }




  // Listen for the Enter key to process commands
  terminalInput.addEventListener('keypress', (event) => {
    if (event.key === 'Enter') {
      let command = terminalInput.value;
      appendLine(`<span style="color: #00ff00;">k></span> ${command}`);
      socket.send(command);
      terminalInput.value = "";

      setTimeout(() => {
        terminalBody.scrollTop = terminalBody.scrollHeight;
      }, 100);

    }
  });

  function prependLine(text) {
    let line = document.createElement("div");
    line.classList.add("line");
    line.innerHTML = text;
    terminalBody.insertBefore(line, terminalInput.parentNode);
  }

  function appendLine(text) {
    let line = document.createElement("div");
    line.classList.add("line");
    line.innerHTML = text;
    terminalBody.appendChild(line);
  }

  function moveToLast(child, parent) {
    if (parent && child) {
      parent.appendChild(child);
    }
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
    socket.close();
    element.remove();
    shellCounter--;
  });

}
