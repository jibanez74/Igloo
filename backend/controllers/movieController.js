import asyncHandler from "../lib/asyncHandler";
import Movie from "../models/Movie";
import ErrorResponse from "../lib/errorResponse";

export const getMovieCount = asyncHandler(async (req, res, next) => {
  const count = await Movie.countDocuments();

  res.status(200).json({
    success: true,
    count,
  });
});

export const getMovieByID = asyncHandler(async (req, res, next) => {
  const movie = await Movie.findById(req.params.id);

  if (!movie) {
    return next(
      new ErrorResponse(`unable to find movie with id ${req.params.id}`, 404)
    );
  }

  res.status(200).json({
    success: true,
    movie,
  });
});

export const getMoviesWithPagination = asyncHandler(async (req, res, next) => {
  const pageSize = 24;
  const page = Number(req.query.pageNumber) || 1;

  const keyword = req.query.keyword
    ? {
        title: {
          $regex: req.query.keyword,
          $options: "i",
        },
      }
    : {};

  const count = await Movie.countDocuments({ ...keyword });

  const movies = await Movie.find({ ...keyword })
    .limit(pageSize)
    .skip(pageSize * (page - 1));

  res.status(200).json({
    success: true,
    movies,
    page,
    pages: Math.ceil(count / pageSize),
  });
});

export const createMovie = asyncHandler(async (req, res, next) => {
  const movie = await Movie.create(req.body);

  res.status(201).json({
    success: true,
    movie,
  });
});
