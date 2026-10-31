import { get } from '../tools';

export async function pluginPlaceGetCountries(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/place/countries',
    params,
    options,
  });
}

export async function pluginPlaceGetStates(
  params?: { country_name?: string; country_code?: string },
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/place/country/states',
    params,
    options,
  });
}

export async function pluginPlaceGetCities(
  params?: {
    country_name?: string;
    state_name?: string;
    country_code?: string;
    state_code?: string;
  },
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/place/country/cities',
    params,
    options,
  });
}
