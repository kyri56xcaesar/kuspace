// Sign-in and register pages: submit the form with fetch, follow the
// redirect on success, show the server's error otherwise.
// (Moved out of inline <script> blocks so the pages run under a strict CSP.)
(() => {
  const FORMS = [
    { form: "login-form", loader: "login-loader", error: "login-error", label: "Login error" },
    { form: "register-form", loader: "register-loader", error: "register-error", label: "Register error" },
  ];

  function setup({ form: formId, loader: loaderId, error: errorId, label }) {
    const form = document.getElementById(formId);
    if (!form) return;
    const loaderDiv = document.getElementById(loaderId);
    const errorDiv = document.getElementById(errorId);
    const submitButton = form.querySelector('[type="submit"]');
    let hideTimer = null;

    // inputs start readonly so browsers do not autofill them; unlock on focus
    form.addEventListener("focusin", (e) => {
      if (e.target.matches("input[readonly]")) e.target.removeAttribute("readonly");
    });

    form.addEventListener("submit", (event) => {
      event.preventDefault();
      if (loaderDiv) loaderDiv.classList.remove("hidden");
      if (submitButton) submitButton.disabled = true;

      let resp;
      fetch(form.action, { method: "POST", body: new FormData(form) })
        .then((response) => {
          resp = response;
          if (!response.ok) {
            return response
              .json()
              .catch(() => ({}))
              .then((data) => {
                throw { status: response.status, message: data.error || "An error occurred." };
              });
          }
          // success: the server redirected us, follow it
          window.location.href = response.url;
        })
        .catch((err) => {
          console.error("Error:", err);
          if (errorDiv) {
            errorDiv.textContent = "";
            const msg = document.createElement("span");
            msg.textContent = `${label}: ${(err && err.message) || "unknown error"}`;
            errorDiv.appendChild(msg);

            // register: 403 means the account already exists
            if (formId === "register-form" && resp && resp.status === 403) {
              const again = document.createElement("button");
              again.type = "button";
              again.textContent = "Is it you? Sign in";
              again.addEventListener("click", () => {
                window.location.href = "/api/v1/login";
              });
              errorDiv.appendChild(again);
            }
            errorDiv.classList.remove("hidden");
          }

          form.reset();
          clearTimeout(hideTimer);
          hideTimer = setTimeout(() => {
            if (errorDiv) {
              errorDiv.classList.add("hidden");
              errorDiv.textContent = "";
            }
          }, 15000);
        })
        .finally(() => {
          if (loaderDiv) loaderDiv.classList.add("hidden");
          if (submitButton) submitButton.disabled = false;
        });
    });
  }

  function init() {
    FORMS.forEach(setup);
  }

  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", init);
  else init();
})();
