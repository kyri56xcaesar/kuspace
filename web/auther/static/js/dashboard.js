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


    const uchartCanvas = document.getElementById("user-df-chart");
    const ctx1 = setupCanvas(uchartCanvas, 250, 250);
    const uchart = new myChart(uchartCanvas, ctx1, {max: 100, step: 5, tick: 10, offset: 3, Rscale: 0.8, rays: [5, 5], fontWidth: 10});
    const layer1 = new Layer(uchart, {max:100, at: 46}, {name:'Layer 1', color:'rgba(66, 164, 177, 0.5)', padding: 25});
    uchart.addLayer(layer1);
    uchart.draw();


    const gchartCanvas = document.getElementById("group-df-chart");
    const ctx2 = setupCanvas(gchartCanvas, 250, 250);
    const gchart = new myChart(gchartCanvas, ctx2, {max: 100, step: 5, tick: 10, offset: 3, Rscale: 0.8, rays: [5, 5], fontWidth: 10});
    const layer2 = new Layer(gchart, {max: 100, at: 67}, {name:'Layer 2', color:'rgba(25, 145, 109, 0.5)', padding: 25});
    gchart.addLayer(layer2);
    gchart.draw();
});
