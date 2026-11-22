// EuroSAT's ten land-use classes, as the model names them and as people read them.

const LABELS: Record<string, string> = {
  AnnualCrop: "Annual crop",
  Forest: "Forest",
  HerbaceousVegetation: "Herbaceous vegetation",
  Highway: "Highway",
  Industrial: "Industrial",
  Pasture: "Pasture",
  PermanentCrop: "Permanent crop",
  Residential: "Residential",
  River: "River",
  SeaLake: "Sea or lake",
};

export const className = (label: string | null | undefined) => (label ? (LABELS[label] ?? label) : "?");

export const DEGRADATIONS: Record<string, string> = {
  none: "None",
  blur: "Blur",
  noise: "Noise",
  haze: "Haze",
  darken: "Darken",
};
