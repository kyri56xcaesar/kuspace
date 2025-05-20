editor = "";



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




    /**************************************************************************/
    // files
    /**************************************************************************/

    // functionality of file upload via drag
    const dashBoardHome = document.getElementById("dashboard-home");
    const dropZone = dashBoardHome.querySelector("#drop-zone");
    const fileInput = dashBoardHome.querySelector("#file");
    const fileBoxContainer = dashBoardHome.querySelector("#file-boxes");
    const submitButton = dashBoardHome.querySelector("#upload-button");
    const fileNameDisplay = dashBoardHome.querySelector("#file-name");

    fileUploadModule = fileUploadContainerFunctionality(
      dropZone,
      fileInput,
      fileBoxContainer,
      submitButton,
      fileNameDisplay
    );

 
    editor = setupJobSubmitter(document.querySelector('#new-job-container-dash'));


    // USER CHART
    const uchartCanvas = document.getElementById("user-df-chart");
    
    const uUsage = parseFloat(uchartCanvas.dataset.usage);
    const uQuota = parseFloat(uchartCanvas.dataset.quota);
    const uColor = uchartCanvas.dataset.color || 'rgba(66, 164, 177, 0.5)';
    const uName = uchartCanvas.dataset.name || 'User Usage';
    

    const Rscale = 0.5;
    const ctx1 = setupCanvas(uchartCanvas, 500, 500);
    const uchart = new myChart(uchartCanvas, ctx1, {
      max: 100,
      step: 10,
      tick: 10,
      offset: 3,
      Rscale: Rscale,
      rays: [5, 5],
      fontWidth: 10
    });

    const at = ((uUsage / uQuota) * 100).toFixed(2); 


    const layer1 = new Layer(uchartCanvas, ctx1, { max: 100, at: at}, {
      name: uName,
      color: uColor,
      padding: 1,
      RScale: Rscale
    });

    
    uchart.addLayer(layer1);
    uchart.draw();


    const gchartCanvas = document.getElementById("group-df-chart");
    const gLayersRaw = gchartCanvas.dataset.layers;
    let gLayers = [];

    try {
      gLayers = JSON.parse(gLayersRaw);
    } catch (err) {
      console.error("Failed to parse group volume layers:", err);
    }


    const ctx2 = setupCanvas(gchartCanvas, 500, 500);
    const gchart = new myChart(gchartCanvas, ctx2, {
      max: 100, // or calculate maxQuota if needed
      step: 25,
      tick: 10,
      offset: 0,
      Rscale: Rscale,
      rays: [5, 5],
      fontWidth: 5
    });

    // Add each layer from .group_volume slice
    gLayers.forEach((layerData, i) => {
      const usage = parseFloat(layerData.usage);
      const quota = parseFloat(layerData.quota);
      const color = layerData.Color || `rgba(${Math.floor(Math.random() * 256)}, ${Math.floor(Math.random() * 256)}, ${Math.floor(Math.random() * 256)}, 0.3)`;
      const name = layerData.gid || `Layer ${i + 1}`;

      const at = ((usage / quota) * 100).toFixed(2); 

      const layer = new Layer(gchartCanvas, ctx2, { max: 100, at: at }, {
        name,
        color,
        padding: 1,
        RScale: Rscale
      });

      gchart.addLayer(layer);
    });

    gchart.draw();



    /**************************************************************************/
    // search bar
    /**************************************************************************/
    const search = document.getElementById("dashboard-search");
    const resultsPopup = document.getElementById("search-results-modal");

    search.value = "";
    search.addEventListener("input", (event) => {
      const query = event.target.value.toLowerCase();
      resultsPopup.innerHTML = "";

      if (!query) {
        resultsPopup.style.display = "none";
        return;
      }
      // find all elements with visible text content (excluding script/style/head/meta/etc)
      const allTextElements = Array.from(document.body.querySelectorAll("*"))
        .filter(el => el.children.length === 0 && el.textContent.trim() !== "");

      let matchCount = 0;

      allTextElements.forEach(el => {
        const text = el.textContent.toLowerCase();
        if (text.includes(query)) {
          matchCount++;
          const original = el.textContent;
          const highlighted = original.replace(
            new RegExp(`(${query})`, "gi"),
            `<mark>$1</mark>`
          );

        const resultItem = document.createElement("div");
        resultItem.innerHTML = highlighted;
        resultItem.style.marginBottom = "8px";
        resultItem.style.cursor = "pointer";

        resultItem.addEventListener("click", () => {
          el.scrollIntoView({ behavior: "smooth", block: "center" });
          el.classList.add("search-target-highlight");
          setTimeout(() => el.classList.remove("search-target-highlight"), 2000);
        });

        resultsPopup.appendChild(resultItem);
  }
      });

      resultsPopup.style.display = matchCount > 0 ? "block" : "none";

    });

});
