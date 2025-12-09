package ar

  import "gocv.io/x/gocv"

  func ApplyOverlay(original gocv.Mat, designImg gocv.Mat, mask gocv.Mat) gocv.Mat {
      // Наложение: cv.AddWeighted или seamlessClone для реализма
      result := gocv.NewMat();
      gocv.SeamlessClone(designImg, original, mask, gocv.Point{}, &result, gocv.CloneNormalClone);
      return result;
  }
