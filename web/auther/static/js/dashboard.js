document.addEventListener("DOMContentLoaded", () => {
    const profileMenu = document.querySelector(".profile-menu");
    const profileButton = document.querySelector(".profile-button");

    profileButton.addEventListener("click", () => {
        profileMenu.classList.toggle("open");
    });

    document.addEventListener("click", (event) => {
        if (!profileMenu.contains(event.target)) {
            profileMenu.classList.remove("open");
        }
    });

    const container = document.querySelectorAll(".collapsible");
    const toggleButton = document.querySelector("#toggle-button-collapse");
    const h1d = document.getElementById("dashboard-h1-title");

    toggleButton.addEventListener("click", () => {
        toggleButton.classList.toggle("collapsed");
        container.forEach(element => {
            element.classList.toggle("collapsed");
        });
        console.log(h1d);
        if (h1d && toggleButton.classList.contains("collapsed")) {
            let h1Width = h1d.offsetWidth;
            toggleButton.style.transform = `translateX(-${h1Width}px)`;
        } else {
            toggleButton.style.transform = `translateX(0)`;
        }
    });
});