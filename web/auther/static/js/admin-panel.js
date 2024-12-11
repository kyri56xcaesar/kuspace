function editUser(userId) {
  alert(`Edit user with ID: ${userId}`);
  // Perform edit action (e.g., open a modal)
}

function deleteUser(userId) {
  if (confirm(`Are you sure you want to delete user with ID: ${userId}?`)) {
    // Perform delete action (e.g., send a delete request)
    alert(`User with ID: ${userId} deleted.`);
  }
}


