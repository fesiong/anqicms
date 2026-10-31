import { get, post } from './tools';

export async function getPlaces(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/place/list',
    params,
    options,
  });
}

export async function getPlaceInfo(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/place/detail',
    params,
    options,
  });
}

export async function savePlace(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/place/detail',
    body,
    options,
  });
}

export async function deletePlace(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/place/delete',
    body,
    options,
  });
}

export async function getPlaceSetting(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/place/setting',
    params,
    options,
  });
}

export async function savePlaceSetting(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/place/setting',
    body,
    options,
  });
}
