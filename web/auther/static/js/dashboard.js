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
    
    const ctx1 = setupCanvas(uchartCanvas, 250, 250);
    const uchart = new myChart(uchartCanvas, ctx1, {
      max: 100,
      step: 10,
      tick: 10,
      offset: 3,
      Rscale: 0.8,
      rays: [5, 5],
      fontWidth: 10
    });
    
    const layer1 = new Layer(uchart, { max: 100, at: (uUsage /  uQuota) * 100}, {
      name: uName,
      color: uColor,
      padding: 25
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


    const ctx2 = setupCanvas(gchartCanvas, 250, 250);
    const gchart = new myChart(gchartCanvas, ctx2, {
      max: 100, // or calculate maxQuota if needed
      step: 25,
      tick: 10,
      offset: 3,
      Rscale: 0.8,
      rays: [5, 5],
      fontWidth: 0
    });

    // Add each layer from .group_volume slice
    gLayers.forEach((layerData, i) => {
      const usage = parseFloat(layerData.usage);
      const quota = parseFloat(layerData.quota);
      const color = layerData.Color || 'rgba(100, 100, 255, 0.3)';
      const name = layerData.Name || `Layer ${i + 1}`;

      const layer = new Layer(gchart, { max: 100, at: (usage / quota) * 100 }, {
        name,
        color,
        padding: 25
      });

      gchart.addLayer(layer);
    });

    gchart.draw();



});
