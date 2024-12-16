
function editUser(uid, index) {
  const row = document.getElementById(`table-${index}`);
  if (!row) return;

  const cells = row.querySelectorAll('td');
  if (!cells) return;

  const originalValues = {};

  for (let i = 1; i < cells.length - 1; i++) {
    const cell = cells[i];
    const originalText = cell.textContent.trim();

    originalValues[i] = originalText;

    const input = document.createElement('input');
    input.type = 'text';
    //input.value = originalText;
    input.placeholder = originalText;
    input.dataset.index = i;
    cell.innerHTML = '';
    cell.appendChild(input);
  }

  const actionsCell = cells[cells.length - 1];
  actionsCell.innerHTML = `
    <div id="actions-btns">
      <button id="submit-btn-${index}" onclick='submitUser(${index}, ${JSON.stringify(originalValues).replace(/'/g, "\\'")})'>Submit</button>
      <button id="cancel-btn-${index}" onclick='cancelEdit(${index}, ${JSON.stringify(originalValues).replace(/'/g, "\\'")})'>Cancel</button>
    </div>
  `;
}

function cancelEdit(index, originalValues) {
  const row = document.getElementById(`table-${index}`);
  if (!row) return;

  const cells = row.querySelectorAll('td');
  for (let i = 1; i < cells.length - 1; i++) {
    cells[i].innerHTML = originalValues[i];
  }

  const actionsCell = cells[cells.length - 1];
  actionsCell.innerHTML = `
    <div id="actions-btns">
      <button id="edit-btn-{{ $index }}" onclick='editUser("", ${index})'>Edit</button>
      <button id="delete-btn-{{ $index }}" onclick="deleteUser('{{ $user.Uid }}', '{{ $index }}')">Delete</button>
    </div>
  `
}

function submitUser(index, originalValues) {
  const row = document.getElementById(`table-${index}`);
  if (!row) return;

  const cells = row.querySelectorAll('td');
  const updatedData = {};

  // Gather updated data from the inputs
  for (let i = 0; i < cells.length - 1; i++) {
    const input = cells[i].querySelector('input');
    if (input) {
      updatedData[input.dataset.index] = input.value || input.placeholder; // Use placeholder if input is empty
    }
  }

  // Ask for confirmation
  if (!confirm('Are you sure you want to update this user?')) {
    cancelEdit(index, originalValues);
    return;
  }

  // Send PATCH request with updated data
  fetch('/api/v1/admin/user/update', {
    method: 'PATCH',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(updatedData),
  })
    .then((response) => {
      if (!response.ok) {
        throw new Error('Failed to update user');
      }
      return response.json();
    })
    .then((data) => {
      alert('User updated successfully!');
      // Reload the page or update the row with the new data
      for (let i = 0; i < cells.length - 1; i++) {
        cells[i].innerHTML = updatedData[i];
      }
      // Restore the Actions column
      const actionsCell = cells[cells.length - 1];
      actionsCell.innerHTML = `
        <button id="edit-btn-${index}" onclick="editUser('${index}')">Edit</button>
      `;
    })
    .catch((err) => {
      console.error(err);
      alert('Failed to update user');
      cancelEdit(index, originalValues);
    });
}

function deleteUser(uid, index) {
  console.log(`Deleting user with UID: ${uid}, Row Index: ${index}`);
  const row = document.getElementById(`table-${index}`);
  if (row) {
    
    alert(`User with ID: ${userId} deleted.`);

    row.remove(); // Example: Remove the row from the table
  }
}

function getCookie(name) {
  const value = `; ${document.cookie}`;
  const parts = value.split(`; ${name}=`);
  if (parts.length === 2) return parts.pop().split(';').shift();
  return '';
}

