document.addEventListener("DOMContentLoaded", () => {
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
});
